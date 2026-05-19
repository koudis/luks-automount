package worker

const (
	OpUnlockAndMount  = "unlock_and_mount"
	OpUnmountAndClose = "unmount_and_close"
	OpReadUUID        = "read_uuid"
)

type Request struct {
	Op         string `json:"op"`
	Dev        string `json:"dev,omitempty"`
	Mapper     string `json:"mapper,omitempty"`
	MountPoint string `json:"mount_point,omitempty"`
	FS         string `json:"fs,omitempty"`
	Options    string `json:"options,omitempty"`
	UID        int    `json:"uid,omitempty"`
	GID        int    `json:"gid,omitempty"`
}
