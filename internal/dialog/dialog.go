package dialog

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"luks-automount/internal/worker"
)

var (
	// ErrNoGraphicalSession reports that no display environment could be derived
	// for launching a GNOME warning dialog.
	ErrNoGraphicalSession = errors.New("no graphical session available")
	// ErrDialogToolNotFound reports that the expected dialog program could not be found.
	ErrDialogToolNotFound = errors.New("GNOME dialog utility not found")
	lookPath              = exec.LookPath
	x11SocketDir          = "/tmp/.X11-unix"
	runCommand            = func(name string, env []string, args ...string) error {
		cmd := exec.Command(name, args...)
		cmd.Env = env
		cmd.Stdout = io.Discard
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			message := strings.TrimSpace(stderr.String())
			if message == "" {
				return err
			}
			return fmt.Errorf("%w: %s", err, message)
		}
		return nil
	}
)

// ShowMountPointBusy displays a GNOME warning dialog for a blocked unmount and
// derives a graphical environment for the user service when needed.
func ShowMountPointBusy(mountPoint string, users []worker.MountUser) error {
	env, ok := graphicalEnvironment()
	if !ok {
		return ErrNoGraphicalSession
	}
	zenity, err := lookPath("zenity")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDialogToolNotFound, err)
	}
	return runCommand(zenity, env, "--warning", "--no-markup", "--title", "Mount point is busy", "--text", MountPointBusyText(mountPoint, users), "--width", "560")
}

// MountPointBusyText formats the dialog body for a mount-point-busy warning.
func MountPointBusyText(mountPoint string, users []worker.MountUser) string {
	var b strings.Builder
	b.WriteString("The mount point ")
	b.WriteString(mountPoint)
	b.WriteString(" is still used by other applications.\n\nClose these applications and retry:")
	if len(users) == 0 {
		b.WriteString("\n\nNo process details are available.")
		return b.String()
	}
	b.WriteString("\n\n")
	b.WriteString(worker.FormatMountUsers(users))
	return b.String()
}

// graphicalEnvironment derives the minimum display-related environment needed
// to start a GUI dialog from a user service by checking explicit env vars,
// runtime-dir Wayland sockets, and the default X11 socket.
func graphicalEnvironment() ([]string, bool) {
	env := os.Environ()
	runtimeDir := envValue(env, "XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		fallback := fmt.Sprintf("/run/user/%d", os.Getuid())
		if isDir(fallback) {
			env = setEnv(env, "XDG_RUNTIME_DIR", fallback)
			runtimeDir = fallback
		}
	}
	if envValue(env, "WAYLAND_DISPLAY") == "" && runtimeDir != "" {
		if display := firstWaylandDisplay(runtimeDir); display != "" {
			env = setEnv(env, "WAYLAND_DISPLAY", display)
		}
	}
	if envValue(env, "DISPLAY") == "" && x11DisplayExists() {
		env = setEnv(env, "DISPLAY", ":0")
		if envValue(env, "XAUTHORITY") == "" {
			if home, err := os.UserHomeDir(); err == nil {
				xauthority := filepath.Join(home, ".Xauthority")
				if fileExists(xauthority) {
					env = setEnv(env, "XAUTHORITY", xauthority)
				}
			}
		}
	}
	return env, envValue(env, "DISPLAY") != "" || envValue(env, "WAYLAND_DISPLAY") != ""
}

// firstWaylandDisplay finds the first usable wayland-* socket in the runtime
// directory so the dialog can recover a missing WAYLAND_DISPLAY.
func firstWaylandDisplay(runtimeDir string) string {
	matches, err := filepath.Glob(filepath.Join(runtimeDir, "wayland-*"))
	if err != nil {
		return ""
	}
	sort.Strings(matches)
	for _, match := range matches {
		if strings.HasSuffix(match, ".lock") {
			continue
		}
		if fileExists(match) {
			return filepath.Base(match)
		}
	}
	return ""
}

func x11DisplayExists() bool {
	return fileExists(filepath.Join(x11SocketDir, "X0"))
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// envValue reads one variable from an environment slice without mutating it.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return strings.TrimPrefix(item, prefix)
		}
	}
	return ""
}

// setEnv updates or appends one variable inside an environment slice.
func setEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}
