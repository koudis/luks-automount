package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func newUninstallServiceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-service",
		Short: "Disable and remove the user systemd service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstallService()
		},
	}
}

func runUninstallService() error {
	unitPath, err := userUnitPath()
	if err != nil {
		return err
	}
	if _, statErr := os.Stat(unitPath); os.IsNotExist(statErr) {
		slog.Warn("unit file not found, nothing to uninstall", "path", unitPath)
		return nil
	}
	if err := systemctlUser("disable", "--now", serviceUnitName); err != nil {
		slog.Warn("systemctl disable --now failed", "err", err)
	}
	if err := os.Remove(unitPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}
	if err := systemctlUser("daemon-reload"); err != nil {
		return err
	}
	fmt.Printf("removed %s\n", unitPath)
	return nil
}
