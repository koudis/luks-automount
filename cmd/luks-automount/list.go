package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List registered disks and their current status",
		Args:  noArgs(),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig()
			if err != nil {
				return err
			}
			mounts := readProcMounts()
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "NAME\tUUID\tDEV\tUNLOCKED\tMOUNTED")
			for i := range cfg.Disks {
				d := &cfg.Disks[i]
				unlocked := mapperExists(d.MapperName)
				dev := mounts[d.MountPoint]
				if dev == "" && unlocked {
					dev = mapperSlaveDev(d.MapperName)
				}
				mounted := dev != "" && mounts[d.MountPoint] != ""
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
					d.Name, d.LUKSUUID, dev,
					yesNo(unlocked), yesNo(mounted))
			}
			return tw.Flush()
		},
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func mapperExists(mapperName string) bool {
	_, err := os.Stat("/dev/mapper/" + mapperName)
	return err == nil
}

func mapperSlaveDev(mapperName string) string {
	target, err := os.Readlink("/dev/mapper/" + mapperName)
	if err != nil {
		return ""
	}
	dmName := filepath.Base(target)
	slavesDir := filepath.Join("/sys/block", dmName, "slaves")
	entries, err := os.ReadDir(slavesDir)
	if err != nil || len(entries) == 0 {
		return ""
	}
	return "/dev/" + entries[0].Name()
}

func readProcMounts() map[string]string {
	out := make(map[string]string)
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return out
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		fields := strings.Fields(s.Text())
		if len(fields) < 2 {
			continue
		}
		out[fields[1]] = fields[0]
	}
	return out
}
