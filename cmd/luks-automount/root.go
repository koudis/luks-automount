package main

import (
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "luks-automount",
		Short:         "Auto-unlock and mount LUKS USB disks",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(
		newAddCmd(),
		newRemoveCmd(),
		newListCmd(),
		newUnlockCmd(),
		newLockCmd(),
		newRunCmd(),
		newInstallServiceCmd(),
		newUninstallServiceCmd(),
		newWorkerCmd(),
	)
	return root
}
