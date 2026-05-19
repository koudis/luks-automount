package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"luks-automount/internal/config"
	"luks-automount/internal/engine"
	"luks-automount/internal/keyring"
	"luks-automount/internal/worker"
)

func newAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Register a new LUKS disk",
		Args:  cobra.ExactArgs(1),
		RunE:  runAdd,
	}
}

func runAdd(cmd *cobra.Command, args []string) error {
	name := args[0]
	if err := config.ValidateName(name); err != nil {
		return err
	}
	d := config.Disk{Name: name, MapperName: name}

	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if cfg.Find(name) != nil {
		return fmt.Errorf("disk %q already registered", name)
	}

	disks, err := engine.ScanPluggedDisks()
	if err != nil {
		return fmt.Errorf("scan plugged disks: %w", err)
	}
	if len(disks) == 0 {
		return fmt.Errorf("no removable USB block devices found; plug the disk and retry")
	}

	fmt.Fprintln(os.Stderr, "Currently plugged removable block devices:")
	for i, p := range disks {
		fmt.Fprintf(os.Stderr, "  [%d] %s\n", i+1, p.DevPath)
	}
	sel, err := readLine("Select device by number: ")
	if err != nil {
		return err
	}
	idx, err := strconv.Atoi(strings.TrimSpace(sel))
	if err != nil || idx < 1 || idx > len(disks) {
		return fmt.Errorf("invalid selection")
	}
	devPath := disks[idx-1].DevPath

	client, err := worker.NewClient()
	if err != nil {
		return err
	}
	uuid, err := client.ReadUUID(devPath)
	if err != nil {
		return fmt.Errorf("read luks uuid from %s: %w", devPath, err)
	}
	d.LUKSUUID = uuid

	mountPoint, err := readLine(fmt.Sprintf("Mount point (must exist under %s/): ", config.MountRoot))
	if err != nil {
		return err
	}
	d.MountPoint = strings.TrimSpace(mountPoint)
	if err := config.ValidateMountPoint(d.MountPoint); err != nil {
		return err
	}
	if st, err := os.Stat(d.MountPoint); err != nil || !st.IsDir() {
		return fmt.Errorf("mount point %s does not exist or is not a directory; create it with: sudo mkdir -p %s && sudo chown $USER %s",
			d.MountPoint, d.MountPoint, d.MountPoint)
	}

	fs, err := readLine(fmt.Sprintf("Filesystem (%s): ", strings.Join(config.SupportedFilesystems, ", ")))
	if err != nil {
		return err
	}
	d.FilesystemType = strings.TrimSpace(fs)

	opts, err := readLine("Mount options (comma-separated, optional): ")
	if err != nil {
		return err
	}
	d.MountOptions = strings.TrimSpace(opts)

	pass, err := readPassphrase("LUKS passphrase: ")
	if err != nil {
		return err
	}
	defer zero(pass)
	if len(pass) == 0 {
		return fmt.Errorf("empty passphrase not allowed")
	}

	if err := cfg.Add(d); err != nil {
		return err
	}
	if err := keyring.Set(name, string(pass)); err != nil {
		_ = cfg.Remove(name)
		return fmt.Errorf("store passphrase in keyring: %w", err)
	}
	if err := cfg.Save(); err != nil {
		_ = keyring.Delete(name)
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Registered %s (uuid=%s)\n", name, uuid)
	return nil
}


