package worker

import (
	"strings"

	"golang.org/x/sys/unix"
)

var flagMap = map[string]uintptr{
	"ro":            unix.MS_RDONLY,
	"rw":            0,
	"nosuid":        unix.MS_NOSUID,
	"nodev":         unix.MS_NODEV,
	"noexec":        unix.MS_NOEXEC,
	"sync":          unix.MS_SYNCHRONOUS,
	"dirsync":       unix.MS_DIRSYNC,
	"noatime":       unix.MS_NOATIME,
	"nodiratime":    unix.MS_NODIRATIME,
	"relatime":      unix.MS_RELATIME,
	"strictatime":   unix.MS_STRICTATIME,
	"mand":          unix.MS_MANDLOCK,
	"silent":        unix.MS_SILENT,
}

func parseMountOptions(opts []string) (uintptr, string) {
	var flags uintptr
	var data []string
	for _, o := range opts {
		if f, ok := flagMap[o]; ok {
			flags |= f
			continue
		}
		data = append(data, o)
	}
	return flags, strings.Join(data, ",")
}
