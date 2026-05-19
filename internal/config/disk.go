package config

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

const MountRoot = "/mnt"

var (
	nameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

var SupportedFilesystems = []string{"ext4", "btrfs", "xfs", "vfat", "exfat", "ntfs"}

type Disk struct {
	Name           string `toml:"name"`
	LUKSUUID       string `toml:"luks_uuid"`
	MapperName     string `toml:"mapper_name"`
	MountPoint     string `toml:"mount_point"`
	FilesystemType string `toml:"filesystem_type"`
	MountOptions   string `toml:"mount_options,omitempty"`
}

func ValidateName(name string) error {
	if !nameRe.MatchString(name) {
		return fmt.Errorf("name %q must match [a-zA-Z0-9_-]+", name)
	}
	return nil
}

func (d *Disk) Validate() error {
	if err := ValidateName(d.Name); err != nil {
		return err
	}
	if !uuidRe.MatchString(d.LUKSUUID) {
		return fmt.Errorf("luks_uuid %q is not a valid UUID", d.LUKSUUID)
	}
	if !nameRe.MatchString(d.MapperName) {
		return fmt.Errorf("mapper_name %q must match [a-zA-Z0-9_-]+", d.MapperName)
	}
	if err := ValidateMountPoint(d.MountPoint); err != nil {
		return err
	}
	if !IsSupportedFilesystem(d.FilesystemType) {
		return fmt.Errorf("filesystem_type %q is not supported; allowed: %s",
			d.FilesystemType, strings.Join(SupportedFilesystems, ", "))
	}
	return nil
}

func ValidateMountPoint(p string) error {
	if p == "" {
		return fmt.Errorf("mount_point is empty")
	}
	cleaned := filepath.Clean(p)
	if cleaned != p {
		return fmt.Errorf("mount_point %q is not in clean form; expected %q", p, cleaned)
	}
	if strings.Contains(p, "..") {
		return fmt.Errorf("mount_point %q must not contain '..'", p)
	}
	if !strings.HasPrefix(cleaned, MountRoot+"/") {
		return fmt.Errorf("mount_point %q must be under %s/", p, MountRoot)
	}
	return nil
}

func IsSupportedFilesystem(fs string) bool {
	for _, s := range SupportedFilesystems {
		if s == fs {
			return true
		}
	}
	return false
}

func IsFATFamily(fs string) bool {
	switch fs {
	case "vfat", "exfat", "ntfs":
		return true
	}
	return false
}
