package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"luks-automount/internal/config"
	"luks-automount/internal/luks"

	"golang.org/x/sys/unix"
)

const mapperPathWaitTimeout = 2 * time.Second

const (
	closeMapperAttempts = 10
)

var (
	lockMapper            = luks.Lock
	unlockMapper          = luks.Unlock
	closeMapperRetryDelay = 500 * time.Millisecond
	statPath              = os.Stat
	readMountTable        = os.ReadFile
	mountFilesystem       = unix.Mount
	unmountFilesystem     = unix.Unmount
	findMountUsers        = FindMountUsers
)

type Server struct {
	in  io.Reader
	out io.Writer
}

func NewServer(in io.Reader, out io.Writer) *Server {
	return &Server{in: in, out: out}
}

func (s *Server) Run() int {
	reader := bufio.NewReader(s.in)
	jsonLine, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		s.writeResponse(false, "read request: "+err.Error())
		return ExitProtocol
	}
	jsonLine = strings.TrimRight(jsonLine, "\n")
	if jsonLine == "" {
		s.writeResponse(false, "empty request")
		return ExitProtocol
	}

	var req Request
	if err := json.Unmarshal([]byte(jsonLine), &req); err != nil {
		s.writeResponse(false, "parse request: "+err.Error())
		return ExitProtocol
	}
	if err := validateRequest(&req); err != nil {
		s.writeResponse(false, err.Error())
		return ExitProtocol
	}

	switch req.Op {
	case OpUnlockAndMount:
		return s.handleUnlockAndMount(&req, reader)
	case OpUnmountAndClose:
		return s.handleUnmountAndClose(&req)
	case OpReadUUID:
		return s.handleReadUUID(&req)
	}
	s.writeResponse(false, "unreachable")
	return ExitProtocol
}

func (s *Server) handleReadUUID(req *Request) int {
	uuid, err := luks.ReadUUID(req.Dev)
	if err != nil {
		s.writeResponse(false, err.Error())
		return ExitOpError
	}
	s.writeResponse(true, uuid)
	return ExitOK
}

func (s *Server) handleUnlockAndMount(req *Request, reader *bufio.Reader) int {
	mapperPath := mapperDevicePath(req.Mapper)
	mountedFrom, err := mountedSource(req.MountPoint)
	if err != nil {
		s.writeResponse(false, "mount state: "+err.Error())
		return ExitOpError
	}
	if mountedFrom == mapperPath {
		s.writeResponse(true, "")
		return ExitOK
	}
	if mountedFrom != "" {
		s.writeResponse(false, fmt.Sprintf("mount point %s is already mounted from %s", req.MountPoint, mountedFrom))
		return ExitOpError
	}

	justUnlocked := false
	if !mapperExists(req.Mapper) {
		passLine, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			s.writeResponse(false, "read passphrase: "+err.Error())
			return ExitProtocol
		}
		pass := []byte(strings.TrimRight(passLine, "\n"))
		defer zero(pass)

		if err := unlockMapper(req.Dev, req.Mapper, pass); err != nil {
			if luks.IsWrongPassphrase(err) {
				s.writeResponse(false, "wrong passphrase")
				return ExitOpError
			}
			s.writeResponse(false, "unlock: "+err.Error())
			return ExitOpError
		}
		justUnlocked = true
		if err := waitForPath(mapperPath, mapperPathWaitTimeout); err != nil {
			_ = lockMapper(req.Mapper)
			s.writeResponse(false, "mapper path: "+err.Error())
			return ExitOpError
		}
	}
	if err := ensureDirectory(req.MountPoint); err != nil {
		if justUnlocked {
			_ = lockMapper(req.Mapper)
		}
		s.writeResponse(false, "mount point: "+err.Error())
		return ExitOpError
	}
	if err := s.mount(mapperPath, req); err != nil {
		if justUnlocked {
			_ = lockMapper(req.Mapper)
		}
		s.writeResponse(false, "mount: "+err.Error())
		return ExitOpError
	}
	if !config.IsFATFamily(req.FS) {
		if err := os.Chown(req.MountPoint, req.UID, req.GID); err != nil {
			slog.Warn("chown failed", "mount_point", req.MountPoint, "err", err)
		}
	}
	s.writeResponse(true, "")
	return ExitOK
}

func (s *Server) handleUnmountAndClose(req *Request) int {
	mapperPath := mapperDevicePath(req.Mapper)
	mountedFrom, err := mountedSource(req.MountPoint)
	if err != nil {
		s.writeResponse(false, "mount state: "+err.Error())
		return ExitOpError
	}
	if mountedFrom != "" && mountedFrom != mapperPath {
		s.writeResponse(false, fmt.Sprintf("mount point %s is already mounted from %s", req.MountPoint, mountedFrom))
		return ExitOpError
	}
	if mountedFrom == mapperPath {
		users, err := findMountUsers(req.MountPoint)
		if err != nil {
			s.writeResponse(false, "mount users: "+err.Error())
			return ExitOpError
		}
		if len(users) > 0 {
			s.writeBusyResponse(req.MountPoint, users)
			return ExitOpError
		}
		if err := unmountFilesystem(req.MountPoint, unix.MNT_DETACH); err != nil && err != unix.EINVAL {
			s.writeResponse(false, "unmount: "+err.Error())
			return ExitOpError
		}
	}
	if mapperExists(req.Mapper) {
		if err := closeMapperWithRetry(req.Mapper); err != nil {
			s.writeResponse(false, "close: "+err.Error())
			return ExitOpError
		}
	}
	s.writeResponse(true, "")
	return ExitOK
}

func (s *Server) mount(source string, req *Request) error {
	kept, dropped := sanitizeOptions(req.Options)
	if len(dropped) > 0 {
		slog.Warn("dropped forbidden mount options", "options", dropped)
	}
	parts := []string{"nosuid", "nodev"}
	if config.IsFATFamily(req.FS) {
		parts = append(parts, fmt.Sprintf("uid=%d", req.UID), fmt.Sprintf("gid=%d", req.GID))
	}
	parts = append(parts, kept...)
	flags, data := parseMountOptions(parts)
	return mountFilesystem(source, req.MountPoint, req.FS, flags, data)
}

func mapperDevicePath(mapper string) string {
	return "/dev/mapper/" + mapper
}

func mapperExists(mapper string) bool {
	_, err := statPath(mapperDevicePath(mapper))
	return err == nil
}

func mountedSource(mountPoint string) (string, error) {
	data, err := readMountTable("/proc/mounts")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if fields[1] == mountPoint {
			return fields[0], nil
		}
	}
	return "", nil
}

func closeMapperWithRetry(mapper string) error {
	var err error
	for attempt := 0; attempt < closeMapperAttempts; attempt++ {
		err = lockMapper(mapper)
		if err == nil {
			return nil
		}
		if !isBusyMapperError(err) {
			return err
		}
		if attempt == closeMapperAttempts-1 {
			break
		}
		time.Sleep(closeMapperRetryDelay)
	}
	return err
}

func isBusyMapperError(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "busy")
}

func waitForPath(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		_, err := statPath(path)
		if err == nil {
			return nil
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("stat %s: %w", path, err)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%s did not appear within %s", path, timeout)
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func ensureDirectory(path string) error {
	st, err := statPath(path)
	if err != nil {
		return err
	}
	if !st.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}
	return nil
}

func (s *Server) writeResponse(ok bool, msg string) {
	resp := Response{OK: ok, Message: msg}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

func (s *Server) writeBusyResponse(mountPoint string, users []MountUser) {
	resp := Response{OK: false, Code: CodeMountPointBusy, Message: busyMessage(mountPoint, len(users)), MountUsers: users}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
