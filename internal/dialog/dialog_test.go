package dialog

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"luks-automount/internal/worker"
)

// TestMountPointBusyText exists to keep the warning useful to the user; it
// proves that by checking that the text includes the mount point, process info,
// and the retry instruction.
func TestMountPointBusyText(t *testing.T) {
	text := MountPointBusyText("/mnt/usb", []worker.MountUser{{PID: 123, Name: "nautilus", Cmdline: "nautilus"}})
	for _, want := range []string{"/mnt/usb", "PID 123 nautilus", "Close these applications"} {
		if !strings.Contains(text, want) {
			t.Fatalf("text %q does not contain %q", text, want)
		}
	}
}

// TestShowMountPointBusyNoGraphicalSession exists to keep the service from
// silently pretending a dialog was shown; it proves that by clearing the GUI
// environment and expecting ErrNoGraphicalSession.
func TestShowMountPointBusyNoGraphicalSession(t *testing.T) {
	originalX11SocketDir := x11SocketDir
	x11SocketDir = t.TempDir()
	t.Cleanup(func() { x11SocketDir = originalX11SocketDir })
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	if err := ShowMountPointBusy("/mnt/usb", nil); !errors.Is(err, ErrNoGraphicalSession) {
		t.Fatalf("got %v, want ErrNoGraphicalSession", err)
	}
}

// TestShowMountPointBusyRunsZenity exists to keep the user-facing dialog call
// stable; it proves that by stubbing zenity and checking the built argv.
func TestShowMountPointBusyRunsZenity(t *testing.T) {
	originalLookPath := lookPath
	originalRunCommand := runCommand
	t.Cleanup(func() {
		lookPath = originalLookPath
		runCommand = originalRunCommand
	})
	t.Setenv("DISPLAY", ":1")

	lookPath = func(name string) (string, error) {
		if name != "zenity" {
			t.Fatalf("unexpected lookup: %s", name)
		}
		return "/usr/bin/zenity", nil
	}
	var gotName string
	var gotArgs []string
	runCommand = func(name string, env []string, args ...string) error {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return nil
	}

	if err := ShowMountPointBusy("/mnt/usb", []worker.MountUser{{PID: 123, Name: "terminal"}}); err != nil {
		t.Fatal(err)
	}
	if gotName != "/usr/bin/zenity" {
		t.Fatalf("got command %q", gotName)
	}
	wantPrefix := []string{"--warning", "--no-markup", "--title", "Mount point is busy", "--text"}
	if len(gotArgs) < len(wantPrefix) || !reflect.DeepEqual(gotArgs[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("got args %v", gotArgs)
	}
}

// TestShowMountPointBusyInfersWaylandDisplay exists because the user service
// may lose WAYLAND_DISPLAY; it proves recovery by exposing only a wayland
// socket in XDG_RUNTIME_DIR and checking the inferred env.
func TestShowMountPointBusyInfersWaylandDisplay(t *testing.T) {
	originalLookPath := lookPath
	originalRunCommand := runCommand
	originalX11SocketDir := x11SocketDir
	t.Cleanup(func() {
		lookPath = originalLookPath
		runCommand = originalRunCommand
		x11SocketDir = originalX11SocketDir
	})
	runtimeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(runtimeDir, "wayland-0"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DISPLAY", "")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	x11SocketDir = t.TempDir()

	lookPath = func(name string) (string, error) {
		return "/usr/bin/zenity", nil
	}
	var gotEnv []string
	runCommand = func(name string, env []string, args ...string) error {
		gotEnv = append([]string(nil), env...)
		return nil
	}

	if err := ShowMountPointBusy("/mnt/usb", nil); err != nil {
		t.Fatal(err)
	}
	if envValue(gotEnv, "WAYLAND_DISPLAY") != "wayland-0" {
		t.Fatalf("WAYLAND_DISPLAY was not inferred from runtime dir: %v", gotEnv)
	}
	if envValue(gotEnv, "XDG_RUNTIME_DIR") != runtimeDir {
		t.Fatalf("unexpected XDG_RUNTIME_DIR in env: %v", gotEnv)
	}
}
