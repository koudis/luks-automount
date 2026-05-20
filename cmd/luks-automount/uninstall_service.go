package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the installed binary, sudoers rule, and user service",
		Args:  noArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstall()
		},
	}
}

func runUninstall() error {
	if os.Geteuid() == 0 {
		return errors.New("run uninstall as the target user; root access is requested through sudo")
	}
	printSudoNotice(os.Stderr)
	steps := []confirmStep{
		{
			prompt: "Disable and remove the user systemd service now?",
			run: func() error {
				return uninstallUserService()
			},
		},
		{
			prompt: fmt.Sprintf("Remove sudoers rule at %s using sudo?", sudoersPath),
			run: func() error {
				return uninstallSudoers()
			},
		},
		{
			prompt: fmt.Sprintf("Remove %s using sudo?", installBinaryPath),
			run: func() error {
				return uninstallBinary()
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

func uninstallUserService() error {
	unitPath, err := userUnitPath()
	if err != nil {
		return err
	}
	if err := systemctlUser("disable", "--now", serviceUnitName); err != nil {
		slog.Warn("systemctl disable --now failed", "err", err)
	}
	if err := os.Remove(unitPath); err != nil {
		if os.IsNotExist(err) {
			slog.Warn("unit file not found, nothing to remove", "path", unitPath)
		} else {
			return fmt.Errorf("remove unit file: %w", err)
		}
	}
	if err := systemctlUser("daemon-reload"); err != nil {
		return err
	}
	return nil
}

func uninstallSudoers() error {
	return runSudo("rm", "-f", sudoersPath)
}

func uninstallBinary() error {
	return runSudo("rm", "-f", installBinaryPath)
}
