package main

import (
	"os"

	"github.com/spf13/cobra"

	"luks-automount/internal/worker"
)

func newWorkerCmd() *cobra.Command {
	return &cobra.Command{
		Use:    worker.SubcommandName,
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			srv := worker.NewServer(os.Stdin, os.Stdout)
			os.Exit(srv.Run())
		},
	}
}
