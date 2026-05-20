package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"luks-automount/internal/engine"
	"luks-automount/internal/logging"
	"luks-automount/internal/worker"
)

const LockFileName = "run.lock"

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run",
		Short: "Run the auto-unlock daemon",
		Args:  noArgs(),
		RunE:  runDaemon,
	}
}

func runDaemon(cmd *cobra.Command, args []string) error {
	if err := logging.Setup(slog.LevelInfo); err != nil {
		return fmt.Errorf("setup logging: %w", err)
	}
	stateDir, err := logging.StateDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir: %w", err)
	}
	release, err := acquireLockFile(filepath.Join(stateDir, LockFileName))
	if err != nil {
		return err
	}
	defer release()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := worker.RunInteractiveSudo(); err != nil {
		slog.Warn("sudo credential refresh failed; subsequent worker calls may prompt", "err", err)
	}
	client, err := worker.NewClient()
	if err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	eng := engine.New(cfg, client)
	slog.Info("luks-automount daemon started", "disks", len(cfg.Disks))
	if err := eng.Run(ctx); err != nil {
		return err
	}
	slog.Info("luks-automount daemon stopped")
	return nil
}

func acquireLockFile(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock file %s: %w", path, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon is already running (lock file %s held)", path)
	}
	return func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		f.Close()
	}, nil
}
