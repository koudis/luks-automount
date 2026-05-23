package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var procRoot = "/proc"

// FindMountUsers scans procfs and returns processes that still reference the
// mount point through cwd, root, exe, file descriptors, or memory maps.
func FindMountUsers(mountPoint string) ([]MountUser, error) {
	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}
	cleanMountPoint := filepath.Clean(mountPoint)
	users := make([]MountUser, 0)
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		procDir := filepath.Join(procRoot, entry.Name())
		if processUsesMountPoint(procDir, cleanMountPoint) {
			users = append(users, MountUser{
				PID:     pid,
				Name:    processName(procDir),
				Cmdline: processCmdline(procDir),
			})
		}
	}
	sort.Slice(users, func(i, j int) bool { return users[i].PID < users[j].PID })
	return users, nil
}

func processUsesMountPoint(procDir, mountPoint string) bool {
	for _, name := range []string{"cwd", "root", "exe"} {
		path, err := os.Readlink(filepath.Join(procDir, name))
		if err == nil && pathUsesMountPoint(path, mountPoint) {
			return true
		}
	}
	if fdUsesMountPoint(filepath.Join(procDir, "fd"), mountPoint) {
		return true
	}
	return mapsUseMountPoint(filepath.Join(procDir, "maps"), mountPoint)
}

func fdUsesMountPoint(fdDir, mountPoint string) bool {
	entries, err := os.ReadDir(fdDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		path, err := os.Readlink(filepath.Join(fdDir, entry.Name()))
		if err == nil && pathUsesMountPoint(path, mountPoint) {
			return true
		}
	}
	return false
}

func mapsUseMountPoint(mapsPath, mountPoint string) bool {
	data, err := os.ReadFile(mapsPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 6 {
			mappedPath := strings.Join(fields[5:], " ")
			if pathUsesMountPoint(mappedPath, mountPoint) {
				return true
			}
		}
	}
	return false
}

func pathUsesMountPoint(path, mountPoint string) bool {
	path = strings.TrimSuffix(path, " (deleted)")
	if !filepath.IsAbs(path) {
		return false
	}
	cleanPath := filepath.Clean(path)
	return cleanPath == mountPoint || strings.HasPrefix(cleanPath, mountPoint+string(os.PathSeparator))
}

func processName(procDir string) string {
	data, err := os.ReadFile(filepath.Join(procDir, "comm"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func processCmdline(procDir string) string {
	data, err := os.ReadFile(filepath.Join(procDir, "cmdline"))
	if err != nil {
		return ""
	}
	raw := strings.TrimRight(string(data), "\x00")
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "\x00")
	if len(parts) == 0 {
		return ""
	}
	executable := strings.TrimSpace(parts[0])
	if executable == "" {
		return ""
	}
	return strings.TrimSpace(filepath.Base(executable))
}

func busyMessage(mountPoint string, count int) string {
	if count == 1 {
		return fmt.Sprintf("mount point %s is used by 1 process", mountPoint)
	}
	return fmt.Sprintf("mount point %s is used by %d processes", mountPoint, count)
}
