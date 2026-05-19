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
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return nil, err
	}
	var disks []PluggedDisk
	for _, e := range entries {
		name := e.Name()
		if !isUSBStorageName(name) {
			continue
		}
		if !isRemovableOrUSB("/sys/block/" + name) {
			continue
		}
		disks = append(disks, PluggedDisk{
			DevPath: "/dev/" + name,
			DevName: name,
		})
	}
	return disks, nil
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
