package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"luks-automount/internal/engine"
	"luks-automount/internal/keyring"
	"luks-automount/internal/worker"
)

func newUnlockCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlock <name>",
		Short: "Unlock and mount a registered disk",
		Args:  exactArgs(1),
		RunE:  runUnlock,
	}
}

func runUnlock(cmd *cobra.Command, args []string) error {
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
	if mountedSource == mapperPath {
		fmt.Printf("unlocked and mounted %s at %s\n", name, d.MountPoint)
		return nil
	}
	if mountedSource != "" {
		return fmt.Errorf("mount point %s is already mounted from %s", d.MountPoint, mountedSource)
	}
	alreadyUnlocked := mapperExists(d.MapperName)

	devPath, err := findPluggedDevByUUID(d.LUKSUUID)
	if err != nil {
		return err
	}

	var passBytes []byte
	if !alreadyUnlocked {
		pass, err := keyring.Get(name)
		if err != nil {
			if !errors.Is(err, keyring.ErrNotFound) {
				return fmt.Errorf("keyring lookup: %w", err)
			}
			p, perr := readPassphrase("LUKS passphrase: ")
			if perr != nil {
				return perr
			}
			passBytes = p
		} else {
			passBytes = []byte(pass)
		}
	}
	defer zero(passBytes)

	client, err := worker.NewClient()
	if err != nil {
		return err
	}
	req := &worker.Request{
		Op:         worker.OpUnlockAndMount,
		Dev:        devPath,
		Mapper:     d.MapperName,
		MountPoint: d.MountPoint,
		FS:         d.FilesystemType,
		Options:    d.MountOptions,
		UID:        os.Getuid(),
		GID:        os.Getgid(),
	}
	if err := client.UnlockAndMount(req, passBytes); err != nil {
		return err
	}
	fmt.Printf("unlocked and mounted %s at %s\n", name, d.MountPoint)
	return nil
}

func findPluggedDevByUUID(uuid string) (string, error) {
	disks, err := engine.ScanPluggedDisks()
	if err != nil {
		return "", err
	}
	client, err := worker.NewClient()
	if err != nil {
		return "", err
	}
	for _, p := range disks {
		got, err := client.ReadUUID(p.DevPath)
		if err != nil {
			continue
		}
		if got == uuid {
			return p.DevPath, nil
		}
	}
	return "", fmt.Errorf("no plugged device matches luks_uuid %s", uuid)
}
