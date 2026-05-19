package worker

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"luks-automount/internal/config"
	"luks-automount/internal/luks"

	"golang.org/x/sys/unix"
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
	passLine, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		s.writeResponse(false, "read passphrase: "+err.Error())
		return ExitProtocol
	}
	pass := []byte(strings.TrimRight(passLine, "\n"))
	defer zero(pass)

	if err := luks.Unlock(req.Dev, req.Mapper, pass); err != nil {
		if luks.IsWrongPassphrase(err) {
			s.writeResponse(false, "wrong passphrase")
			return ExitOpError
		}
		s.writeResponse(false, "unlock: "+err.Error())
		return ExitOpError
	}

	mapperPath := "/dev/mapper/" + req.Mapper
	if err := s.mount(mapperPath, req); err != nil {
		_ = luks.Lock(req.Mapper)
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
	var firstErr error
	if err := unix.Unmount(req.MountPoint, unix.MNT_DETACH); err != nil && err != unix.EINVAL {
		firstErr = fmt.Errorf("unmount: %w", err)
	}
	if err := luks.Lock(req.Mapper); err != nil {
		if firstErr == nil {
			firstErr = fmt.Errorf("close: %w", err)
		}
	}
	if firstErr != nil {
		s.writeResponse(false, firstErr.Error())
		return ExitOpError
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
	return unix.Mount(source, req.MountPoint, req.FS, flags, data)
}

func (s *Server) writeResponse(ok bool, msg string) {
	resp := Response{OK: ok, Message: msg}
	enc := json.NewEncoder(s.out)
	_ = enc.Encode(resp)
}

func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
