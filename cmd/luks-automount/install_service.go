package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"luks-automount/internal/worker"
)

const (
	serviceUnitName   = "luks-automount.service"
	serviceDirPerm    = 0o700
	serviceFilePerm   = 0o644
	installBinaryPath = "/usr/local/bin/luks-automount"
	sudoersPath       = "/etc/sudoers.d/luks-automount"
	sudoersFilePerm   = 0o440
)

const unitTemplate = `[Unit]
Description=LUKS automount daemon
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
Environment=XDG_RUNTIME_DIR=%%t
ExecStart=%s run
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
`

type confirmStep struct {
	prompt string
	run    func() error
}

func newInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the binary, user service, and sudoers rule",
		Args:  noArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall()
		},
	}
}

func runInstall() error {
	if os.Geteuid() == 0 {
		return errors.New("run install as the target user; root access is requested through sudo")
	}
	printSudoNotice(os.Stderr)
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self binary: %w", err)
	}
	username, err := installUsername()
	if err != nil {
		return err
	}
	steps := []confirmStep{
		{
			prompt: fmt.Sprintf("Install %s to %s using sudo?", self, installBinaryPath),
			run: func() error {
				return installBinary(self)
			},
		},
		{
			prompt: fmt.Sprintf("Install sudoers rule at %s using sudo?", sudoersPath),
			run: func() error {
				return installSudoers(username)
			},
		},
		{
			prompt: "Create or update the user systemd service and enable it now?",
			run: func() error {
				return installUserService()
			},
		},
	}
	for _, step := range steps {
		ok, err := confirmStepChoice(step.prompt)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if err := step.run(); err != nil {
			return err
		}
	}
	return nil
}

func installBinary(source string) error {
	if sameFile(source, installBinaryPath) {
		return nil
	}
	if err := runSudo("install", "-d", "-m", "0755", filepath.Dir(installBinaryPath)); err != nil {
		return err
	}
	return runSudo("install", "-m", "0755", source, installBinaryPath)
}

func installUserService() error {
	unitPath, err := userUnitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), serviceDirPerm); err != nil {
		return err
	}
	content := serviceUnitContent(installBinaryPath)
	if err := os.WriteFile(unitPath, []byte(content), serviceFilePerm); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	if err := systemctlUser("daemon-reload"); err != nil {
		return err
	}
	return systemctlUser("enable", "--now", serviceUnitName)
}

func installSudoers(username string) error {
	tmp, err := os.CreateTemp("", "luks-automount-sudoers-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(sudoersContent(username)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := runSudo("visudo", "-cf", tmpPath); err != nil {
		return err
	}
	return runSudo("install", "-m", fmt.Sprintf("%04o", sudoersFilePerm), tmpPath, sudoersPath)
}

func printSudoNotice(w io.Writer) {
	fmt.Fprintln(w, "This command uses sudo for privileged steps. Your sudo password may be required.")
}

func confirmStepChoice(prompt string) (bool, error) {
	for {
		answer, err := readLine(prompt + " [y/N]: ")
		if err != nil {
			return false, err
		}
		switch strings.ToLower(strings.TrimSpace(answer)) {
		case "", "n", "no":
			return false, nil
		case "y", "yes":
			return true, nil
		}
	}
}

func installUsername() (string, error) {
	for _, key := range []string{"USER", "LOGNAME"} {
		username := strings.TrimSpace(os.Getenv(key))
		if validSudoersUsername(username) {
			return username, nil
		}
	}
	return "", errors.New("current username is not valid for a sudoers rule")
}

func validSudoersUsername(username string) bool {
	if username == "" {
		return false
	}
	for _, r := range username {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func serviceUnitContent(binaryPath string) string {
	return fmt.Sprintf(unitTemplate, binaryPath)
}

func sudoersContent(username string) string {
	return fmt.Sprintf("%s ALL=(root) NOPASSWD: %s %s\n", username, installBinaryPath, worker.SubcommandName)
}

func sameFile(left, right string) bool {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false
	}
	return os.SameFile(leftInfo, rightInfo)
}

func runSudo(args ...string) error {
	return runCommand("sudo", args...)
}

func userUnitPath() (string, error) {
	cfgDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfgDir, "systemd", "user", serviceUnitName), nil
}

func systemctlUser(args ...string) error {
	full := append([]string{"--user"}, args...)
	return runCommand("systemctl", full...)
}

func runCommand(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
