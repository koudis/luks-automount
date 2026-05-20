package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"luks-automount/internal/keyring"
)

func newRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a registered disk (refuses if currently mounted)",
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
			mounts := readProcMounts()
			if _, mounted := mounts[d.MountPoint]; mounted {
				return fmt.Errorf("disk %q is currently mounted at %s; run `luks-automount lock %s` first",
					name, d.MountPoint, name)
			}
			if !cfg.Remove(name) {
				return fmt.Errorf("disk %q not found", name)
			}
			if err := cfg.Save(); err != nil {
				return fmt.Errorf("save config: %w", err)
			}
			if err := keyring.Delete(name); err != nil && !errors.Is(err, keyring.ErrNotFound) {
				return fmt.Errorf("delete keyring entry: %w", err)
			}
			fmt.Printf("removed %s\n", name)
			return nil
		},
	}
}
