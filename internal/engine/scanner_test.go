package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsUSBStorageName(t *testing.T) {
	valid := []string{"sda", "sdb", "sdz", "sdab", "sdbc"}
	for _, n := range valid {
		if !isUSBStorageName(n) {
			t.Errorf("isUSBStorageName(%q) = false, want true", n)
		}
	}
	invalid := []string{
		"sd",      // too short
		"sda1",    // has digit
		"nvme0n1", // not sd prefix
		"hda",     // not sd prefix
		"sda_",    // underscore
		"SDA",     // uppercase
		"",
	}
	for _, n := range invalid {
		if isUSBStorageName(n) {
			t.Errorf("isUSBStorageName(%q) = true, want false", n)
		}
	}
}

func TestIsRemovableOrUSB_Removable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "removable"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isRemovableOrUSB(dir) {
		t.Error("expected removable=1 to be detected as removable")
	}
}

func TestIsRemovableOrUSB_NotRemovable(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "removable"), []byte("0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isRemovableOrUSB(dir) {
		t.Error("expected removable=0 to not be detected as removable")
	}
}

func TestIsRemovableOrUSB_USBSymlink(t *testing.T) {
	dir := t.TempDir()
	target := "/sys/devices/pci0000:00/usb1/1-1/host0/target0:0:0/0:0:0:0/block/sda"
	linkPath := filepath.Join(dir, "sda-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}
	if !isRemovableOrUSB(linkPath) {
		t.Error("expected symlink containing /usb to be detected as USB")
	}
}

func TestIsRemovableOrUSB_NoUSBInSymlink(t *testing.T) {
	dir := t.TempDir()
	target := "/sys/devices/pci0000:00/ata1/host0/target0:0:0/0:0:0:0/block/sda"
	linkPath := filepath.Join(dir, "sda-link")
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatal(err)
	}
	if isRemovableOrUSB(linkPath) {
		t.Error("expected symlink without /usb to not be detected as USB")
	}
}

func TestIsRemovableOrUSB_NoFilesNoSymlink(t *testing.T) {
	dir := t.TempDir()
	if isRemovableOrUSB(dir) {
		t.Error("expected dir with no removable file and no symlink to return false")
	}
}

func TestScanPluggedDisksIncludesPartitions(t *testing.T) {
	sysBlock := t.TempDir()
	diskPath := filepath.Join(sysBlock, "sdb")
	partPath := filepath.Join(diskPath, "sdb1")
	if err := os.MkdirAll(partPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(diskPath, "removable"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(partPath, "partition"), []byte("1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	disks, err := scanPluggedDisks(sysBlock)
	if err != nil {
		t.Fatal(err)
	}
	if len(disks) != 2 {
		t.Fatalf("expected disk and partition, got %+v", disks)
	}
	if disks[0].DevPath != "/dev/sdb" || disks[1].DevPath != "/dev/sdb1" {
		t.Fatalf("unexpected disks: %+v", disks)
	}
}
