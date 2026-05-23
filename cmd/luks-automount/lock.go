package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"luks-automount/internal/dialog"
	"luks-automount/internal/worker"
)

var showLockMountPointBusy = dialog.ShowMountPointBusy

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock <name>",
		Short: "Unmount and close a registered disk",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			d := cfg.Find(name)
			if d == nil {
				return fmt.Errorf("disk %q not found", name)
			}
			mapperPath := "/dev/mapper/" + d.MapperName
			mountedSource := readProcMounts()[d.MountPoint]
			if mountedSource != "" && mountedSource != mapperPath {
				return fmt.Errorf("mount point %s is already mounted from %s", d.MountPoint, mountedSource)
			}
			if mountedSource == "" && !mapperExists(d.MapperName) {
				fmt.Printf("unmounted and closed %s\n", name)
				return nil
			}
			client, err := worker.NewClient()
			if err != nil {
				return err
			}
			req := &worker.Request{
				Op:         worker.OpUnmountAndClose,
				Mapper:     d.MapperName,
				MountPoint: d.MountPoint,
			}
			if err := client.UnmountAndClose(req); err != nil {
				return handleBusyLockError(d.MountPoint, err)
			}
			fmt.Printf("unmounted and closed %s\n", name)
			return nil
		},
	}
}

func handleBusyLockError(mountPoint string, err error) error {
	var busy *worker.MountPointBusyError
	if !errors.As(err, &busy) {
		return err
	}
	if dialogErr := showLockMountPointBusy(mountPoint, busy.Users); dialogErr != nil {
		return fmt.Errorf("%w; GNOME warning unavailable: %v", err, dialogErr)
	}
	return err
}
