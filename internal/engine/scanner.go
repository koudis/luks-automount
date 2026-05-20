package engine

import (
	"os"
	"path/filepath"
	"strings"
)

type PluggedDisk struct {
	DevPath string
	DevName string
}

func ScanPluggedDisks() ([]PluggedDisk, error) {
	return scanPluggedDisks("/sys/block")
}

func scanPluggedDisks(sysBlock string) ([]PluggedDisk, error) {
	entries, err := os.ReadDir(sysBlock)
	if err != nil {
		return nil, err
	}
	var disks []PluggedDisk
	for _, e := range entries {
		name := e.Name()
		if !isUSBStorageName(name) {
			continue
		}
		sysPath := filepath.Join(sysBlock, name)
		if !isRemovableOrUSB(sysPath) {
			continue
		}
		disks = append(disks, pluggedDisk(name))
		disks = append(disks, scanPartitions(sysPath)...)
	}
	return disks, nil
}

func scanPartitions(sysPath string) []PluggedDisk {
	entries, err := os.ReadDir(sysPath)
	if err != nil {
		return nil
	}
	var partitions []PluggedDisk
	for _, e := range entries {
		name := e.Name()
		if !isPartitionSysEntry(filepath.Join(sysPath, name)) {
			continue
		}
		partitions = append(partitions, pluggedDisk(name))
	}
	return partitions
}

func pluggedDisk(name string) PluggedDisk {
	return PluggedDisk{
		DevPath: "/dev/" + name,
		DevName: name,
	}
}

func isUSBStorageName(name string) bool {
	if !strings.HasPrefix(name, "sd") {
		return false
	}
	if len(name) < 3 {
		return false
	}
	for _, c := range name[2:] {
		if c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

func isRemovableOrUSB(sysPath string) bool {
	if b, err := os.ReadFile(filepath.Join(sysPath, "removable")); err == nil {
		if strings.TrimSpace(string(b)) == "1" {
			return true
		}
	}
	target, err := os.Readlink(sysPath)
	if err != nil {
		return false
	}
	return strings.Contains(target, "/usb")
}

func isPartitionSysEntry(sysPath string) bool {
	_, err := os.ReadFile(filepath.Join(sysPath, "partition"))
	return err == nil
}
