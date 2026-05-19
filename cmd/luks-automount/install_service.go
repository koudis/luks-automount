package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	serviceUnitName = "luks-automount.service"
	serviceDirPerm  = 0o700
	serviceFilePerm = 0o644
)

const unitTemplate = `[Unit]
Description=LUKS automount daemon
After=graphical-session.target
PartOf=graphical-session.target

[Service]
Type=simple
ExecStart=%s run
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=default.target
`

func newInstallServiceCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "install-service",
		Short: "Install and enable the user systemd service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallService(force)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite existing unit file and restart the service")
	return cmd
}

func runInstallService(force bool) error {
	unitPath, err := userUnitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(unitPath); err == nil && !force {
		return fmt.Errorf("unit file %s already exists; pass --force to overwrite", unitPath)
	}
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate self binary: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(unitPath), serviceDirPerm); err != nil {
		return err
	}
	content := fmt.Sprintf(unitTemplate, self)
	if err := os.WriteFile(unitPath, []byte(content), serviceFilePerm); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	if err := systemctlUser("daemon-reload"); err != nil {
		return err
	}
	if force {
		if err := systemctlUser("restart", serviceUnitName); err != nil {
			return err
		}
	}
	if err := systemctlUser("enable", "--now", serviceUnitName); err != nil {
		return err
	}
	fmt.Printf("installed %s\n", unitPath)
	return nil
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
	c := exec.Command("systemctl", full...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if err := c.Run(); err != nil {
		return fmt.Errorf("systemctl --user %v: %w", args, err)
	}
	return nil
}
