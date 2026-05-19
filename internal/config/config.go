package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	ConfigDirName  = "luks-automount"
	ConfigFileName = "config.toml"
	FilePerm       = 0o600
	DirPerm        = 0o700
)

type Config struct {
	Disks []Disk `toml:"disk"`
	path  string
}

func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, ConfigDirName, ConfigFileName), nil
}

func Load(path string) (*Config, error) {
	c := &Config{path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := toml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config %s: %w", path, err)
	}
	return c, nil
}

func (c *Config) Path() string {
	return c.path
}

func (c *Config) Validate() error {
	names := make(map[string]bool, len(c.Disks))
	uuids := make(map[string]bool, len(c.Disks))
	mappers := make(map[string]bool, len(c.Disks))
	for i := range c.Disks {
		d := &c.Disks[i]
		if err := d.Validate(); err != nil {
			return fmt.Errorf("disk[%d]: %w", i, err)
		}
		if names[d.Name] {
			return fmt.Errorf("duplicate name %q", d.Name)
		}
		if uuids[d.LUKSUUID] {
			return fmt.Errorf("duplicate luks_uuid %q", d.LUKSUUID)
		}
		if mappers[d.MapperName] {
			return fmt.Errorf("duplicate mapper_name %q", d.MapperName)
		}
		names[d.Name] = true
		uuids[d.LUKSUUID] = true
		mappers[d.MapperName] = true
	}
	return nil
}

func (c *Config) Find(name string) *Disk {
	for i := range c.Disks {
		if c.Disks[i].Name == name {
			return &c.Disks[i]
		}
	}
	return nil
}

func (c *Config) FindByUUID(uuid string) *Disk {
	for i := range c.Disks {
		if c.Disks[i].LUKSUUID == uuid {
			return &c.Disks[i]
		}
	}
	return nil
}

func (c *Config) Add(d Disk) error {
	for i := range c.Disks {
		e := &c.Disks[i]
		if e.Name == d.Name {
			return fmt.Errorf("duplicate name %q", d.Name)
		}
		if e.LUKSUUID == d.LUKSUUID {
			return fmt.Errorf("duplicate luks_uuid %q (already used by %q)", d.LUKSUUID, e.Name)
		}
		if e.MapperName == d.MapperName {
			return fmt.Errorf("duplicate mapper_name %q (already used by %q)", d.MapperName, e.Name)
		}
	}
	if err := d.Validate(); err != nil {
		return err
	}
	c.Disks = append(c.Disks, d)
	return nil
}

func (c *Config) Remove(name string) bool {
	for i := range c.Disks {
		if c.Disks[i].Name == name {
			c.Disks = append(c.Disks[:i], c.Disks[i+1:]...)
			return true
		}
	}
	return false
}

func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.path), DirPerm); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(c.path), ".config-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(FilePerm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(c); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, c.path)
}
