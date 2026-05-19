package config_test

import (
	"path/filepath"
	"testing"

	"luks-automount/internal/config"
)

const validUUID = "12345678-1234-1234-1234-123456789abc"

func validDisk(name string) config.Disk {
	return config.Disk{
		Name:           name,
		LUKSUUID:       validUUID,
		MapperName:     name,
		MountPoint:     "/mnt/" + name,
		FilesystemType: "ext4",
	}
}

func TestValidateName(t *testing.T) {
	valid := []string{"disk1", "my-disk", "my_disk", "D"}
	for _, n := range valid {
		if err := config.ValidateName(n); err != nil {
			t.Errorf("ValidateName(%q) unexpected error: %v", n, err)
		}
	}
	invalid := []string{"", "my disk", "my.disk", "my/disk", "has space"}
	for _, n := range invalid {
		if err := config.ValidateName(n); err == nil {
			t.Errorf("ValidateName(%q) expected error, got nil", n)
		}
	}
}

func TestValidateMountPoint(t *testing.T) {
	valid := []string{"/mnt/usb", "/mnt/my-disk", "/mnt/a/b"}
	for _, p := range valid {
		if err := config.ValidateMountPoint(p); err != nil {
			t.Errorf("ValidateMountPoint(%q) unexpected error: %v", p, err)
		}
	}
	invalid := []string{
		"",
		"/mnt",
		"/mntfoo",
		"/tmp/usb",
		"/mnt/../etc",
		"relative/path",
	}
	for _, p := range invalid {
		if err := config.ValidateMountPoint(p); err == nil {
			t.Errorf("ValidateMountPoint(%q) expected error, got nil", p)
		}
	}
}

func TestIsSupportedFilesystem(t *testing.T) {
	for _, fs := range config.SupportedFilesystems {
		if !config.IsSupportedFilesystem(fs) {
			t.Errorf("IsSupportedFilesystem(%q) = false, want true", fs)
		}
	}
	if config.IsSupportedFilesystem("zfs") {
		t.Error("IsSupportedFilesystem(\"zfs\") = true, want false")
	}
}

func TestIsFATFamily(t *testing.T) {
	fat := []string{"vfat", "exfat", "ntfs"}
	for _, fs := range fat {
		if !config.IsFATFamily(fs) {
			t.Errorf("IsFATFamily(%q) = false, want true", fs)
		}
	}
	nonFAT := []string{"ext4", "btrfs", "xfs"}
	for _, fs := range nonFAT {
		if config.IsFATFamily(fs) {
			t.Errorf("IsFATFamily(%q) = true, want false", fs)
		}
	}
}

func TestDiskValidate(t *testing.T) {
	d := validDisk("test")
	if err := d.Validate(); err != nil {
		t.Fatalf("valid disk failed: %v", err)
	}

	bad := validDisk("test")
	bad.LUKSUUID = "not-a-uuid"
	if err := bad.Validate(); err == nil {
		t.Error("expected error for bad UUID")
	}

	bad = validDisk("test")
	bad.FilesystemType = "zfs"
	if err := bad.Validate(); err == nil {
		t.Error("expected error for unsupported filesystem")
	}
}

func TestConfigAddDuplicates(t *testing.T) {
	cfg := &config.Config{}
	d := validDisk("alpha")

	if err := cfg.Add(d); err != nil {
		t.Fatalf("first Add failed: %v", err)
	}
	if err := cfg.Add(d); err == nil {
		t.Error("expected error for duplicate name")
	}

	d2 := validDisk("beta")
	d2.LUKSUUID = validUUID
	if err := cfg.Add(d2); err == nil {
		t.Error("expected error for duplicate UUID")
	}

	d3 := validDisk("gamma")
	d3.MapperName = "alpha"
	if err := cfg.Add(d3); err == nil {
		t.Error("expected error for duplicate mapper name")
	}
}

func TestConfigTOMLRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if err := cfg.Add(validDisk("usb1")); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	cfg2, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if len(cfg2.Disks) != 1 || cfg2.Disks[0].Name != "usb1" {
		t.Fatalf("unexpected disks after round-trip: %+v", cfg2.Disks)
	}
}
