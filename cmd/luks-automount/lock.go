package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"luks-automount/internal/worker"
)

func newLockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lock <name>",
		Short: "Unmount and close a registered disk",
		Args:  cobra.ExactArgs(1),
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
				return err
			}
			fmt.Printf("unmounted and closed %s\n", name)
			return nil
		},
	}
}
