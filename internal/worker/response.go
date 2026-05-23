package worker

import (
	"fmt"
	"strings"
)

const (
	ExitOK       = 0
	ExitOpError  = 1
	ExitProtocol = 2

	// CodeMountPointBusy marks a worker response that aborted before unmounting
	// because one or more processes still use the mount point.
	CodeMountPointBusy = "mount_point_busy"
)

// MountUser describes one process that still references the mount point.
type MountUser struct {
	PID     int    `json:"pid"`
	Name    string `json:"name"`
	Cmdline string `json:"cmdline,omitempty"`
}

// Response is the JSON payload exchanged with the privileged worker.
type Response struct {
	OK         bool        `json:"ok"`
	Message    string      `json:"message"`
	Code       string      `json:"code,omitempty"`
	MountUsers []MountUser `json:"mount_users,omitempty"`
}

// MountPointBusyError reports that unmounting was blocked by active users of a
// mount point and carries the discovered process details.
type MountPointBusyError struct {
	MountPoint string
	Message    string
	Users      []MountUser
}

func (e *MountPointBusyError) Error() string {
	message := strings.TrimSpace(e.Message)
	if message == "" {
		message = fmt.Sprintf("mount point %s is busy", e.MountPoint)
	}
	if len(e.Users) == 0 {
		return message
	}
	return message + ":\n" + FormatMountUsers(e.Users)
}

// FormatMountUsers renders a process list in a user-facing multi-line format.
func FormatMountUsers(users []MountUser) string {
	var b strings.Builder
	for i, u := range users {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(fmt.Sprintf("PID %d", u.PID))
		if u.Name != "" {
			b.WriteString(" ")
			b.WriteString(u.Name)
		}
		if u.Cmdline != "" {
			b.WriteString(" — ")
			b.WriteString(u.Cmdline)
		}
	}
	return b.String()
}
