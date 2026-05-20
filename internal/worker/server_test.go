package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func runServer(input string) (Response, int) {
	in := strings.NewReader(input)
	var out bytes.Buffer
	code := NewServer(in, &out).Run()
	var resp Response
	_ = json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp)
	return resp, code
}

func TestServer_EmptyRequest(t *testing.T) {
	resp, code := runServer("")
	if code != ExitProtocol {
		t.Errorf("expected ExitProtocol, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestServer_BadJSON(t *testing.T) {
	resp, code := runServer("not json\n")
	if code != ExitProtocol {
		t.Errorf("expected ExitProtocol, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestServer_UnknownOp(t *testing.T) {
	req := Request{Op: "do_magic"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitProtocol {
		t.Errorf("expected ExitProtocol, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestServer_ReadUUID_ValidationFail(t *testing.T) {
	req := Request{Op: OpReadUUID, Dev: "/dev/disk/by-id/id"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitProtocol {
		t.Errorf("expected ExitProtocol for validation failure, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestServer_ReadUUID_DevNotFound(t *testing.T) {
	req := Request{Op: OpReadUUID, Dev: "/dev/sdnonexistent"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitOpError {
		t.Errorf("expected ExitOpError for missing device, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestServer_UnlockAndMount_ValidationFail(t *testing.T) {
	req := Request{
		Op:         OpUnlockAndMount,
		Dev:        "/dev/disk/by-id/id",
		Mapper:     "mapper",
		MountPoint: "/mnt/test",
		FS:         "ext4",
	}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitProtocol {
		t.Errorf("expected ExitProtocol, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
	}
}

func TestServer_UnmountAndClose_NotMounted(t *testing.T) {
	originalStatPath := statPath
	originalReadMountTable := readMountTable
	t.Cleanup(func() {
		statPath = originalStatPath
		readMountTable = originalReadMountTable
	})
	statPath = func(path string) (os.FileInfo, error) {
		return nil, os.ErrNotExist
	}
	readMountTable = func(path string) ([]byte, error) {
		return nil, nil
	}

	req := Request{
		Op:         OpUnmountAndClose,
		Mapper:     "no-such-mapper",
		MountPoint: "/mnt/no-such-path",
	}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitOK {
		t.Errorf("expected ExitOK, got %d", code)
	}
	if !resp.OK {
		t.Error("expected ok=true")
	}
}

func TestServer_UnlockAndMount_AlreadyMounted(t *testing.T) {
	originalUnlockMapper := unlockMapper
	originalMountFilesystem := mountFilesystem
	originalReadMountTable := readMountTable
	t.Cleanup(func() {
		unlockMapper = originalUnlockMapper
		mountFilesystem = originalMountFilesystem
		readMountTable = originalReadMountTable
	})

	unlockCalls := 0
	mountCalls := 0
	unlockMapper = func(devPath, mapperName string, passphrase []byte) error {
		unlockCalls++
		return nil
	}
	mountFilesystem = func(source, target, fstype string, flags uintptr, data string) error {
		mountCalls++
		return nil
	}
	readMountTable = func(path string) ([]byte, error) {
		return []byte("/dev/mapper/mapper /mnt/test ext4 rw 0 0\n"), nil
	}

	req := Request{Op: OpUnlockAndMount, Dev: "/dev/sdb1", Mapper: "mapper", MountPoint: "/mnt/test", FS: "ext4"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\nsecret\n")
	if code != ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if unlockCalls != 0 {
		t.Fatalf("expected no unlock calls, got %d", unlockCalls)
	}
	if mountCalls != 0 {
		t.Fatalf("expected no mount calls, got %d", mountCalls)
	}
}

func TestServer_UnlockAndMount_AlreadyOpen(t *testing.T) {
	originalUnlockMapper := unlockMapper
	originalMountFilesystem := mountFilesystem
	originalReadMountTable := readMountTable
	originalStatPath := statPath
	t.Cleanup(func() {
		unlockMapper = originalUnlockMapper
		mountFilesystem = originalMountFilesystem
		readMountTable = originalReadMountTable
		statPath = originalStatPath
	})

	dirInfo, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	mapperFile, err := os.CreateTemp(t.TempDir(), "mapper")
	if err != nil {
		t.Fatal(err)
	}
	defer mapperFile.Close()
	mapperInfo, err := mapperFile.Stat()
	if err != nil {
		t.Fatal(err)
	}

	unlockCalls := 0
	mountCalls := 0
	unlockMapper = func(devPath, mapperName string, passphrase []byte) error {
		unlockCalls++
		return nil
	}
	mountFilesystem = func(source, target, fstype string, flags uintptr, data string) error {
		mountCalls++
		return nil
	}
	readMountTable = func(path string) ([]byte, error) {
		return nil, nil
	}
	statPath = func(path string) (os.FileInfo, error) {
		switch path {
		case "/dev/mapper/mapper":
			return mapperInfo, nil
		case "/mnt/test":
			return dirInfo, nil
		default:
			return nil, os.ErrNotExist
		}
	}

	req := Request{Op: OpUnlockAndMount, Dev: "/dev/sdb1", Mapper: "mapper", MountPoint: "/mnt/test", FS: "ext4"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\nsecret\n")
	if code != ExitOK {
		t.Fatalf("expected ExitOK, got %d", code)
	}
	if !resp.OK {
		t.Fatal("expected ok=true")
	}
	if unlockCalls != 0 {
		t.Fatalf("expected no unlock calls, got %d", unlockCalls)
	}
	if mountCalls != 1 {
		t.Fatalf("expected one mount call, got %d", mountCalls)
	}
}

func TestServer_UnmountAndClose_ForeignMount(t *testing.T) {
	originalLockMapper := lockMapper
	originalUnmountFilesystem := unmountFilesystem
	originalReadMountTable := readMountTable
	t.Cleanup(func() {
		lockMapper = originalLockMapper
		unmountFilesystem = originalUnmountFilesystem
		readMountTable = originalReadMountTable
	})

	closeCalls := 0
	unmountCalls := 0
	lockMapper = func(mapper string) error {
		closeCalls++
		return nil
	}
	unmountFilesystem = func(target string, flags int) error {
		unmountCalls++
		return nil
	}
	readMountTable = func(path string) ([]byte, error) {
		return []byte("/dev/sda1 /mnt/test ext4 rw 0 0\n"), nil
	}

	req := Request{Op: OpUnmountAndClose, Mapper: "mapper", MountPoint: "/mnt/test"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitOpError {
		t.Fatalf("expected ExitOpError, got %d", code)
	}
	if resp.OK {
		t.Fatal("expected ok=false")
	}
	if unmountCalls != 0 {
		t.Fatalf("expected no unmount calls, got %d", unmountCalls)
	}
	if closeCalls != 0 {
		t.Fatalf("expected no close calls, got %d", closeCalls)
	}
}

func TestServer_UnmountAndClose_DoesNotCloseAfterUnmountFailure(t *testing.T) {
	originalLockMapper := lockMapper
	originalUnmountFilesystem := unmountFilesystem
	originalReadMountTable := readMountTable
	originalStatPath := statPath
	t.Cleanup(func() {
		lockMapper = originalLockMapper
		unmountFilesystem = originalUnmountFilesystem
		readMountTable = originalReadMountTable
		statPath = originalStatPath
	})

	mapperFile, err := os.CreateTemp(t.TempDir(), "mapper")
	if err != nil {
		t.Fatal(err)
	}
	defer mapperFile.Close()
	mapperInfo, err := mapperFile.Stat()
	if err != nil {
		t.Fatal(err)
	}

	closeCalls := 0
	lockMapper = func(mapper string) error {
		closeCalls++
		return nil
	}
	unmountFilesystem = func(target string, flags int) error {
		return errors.New("busy")
	}
	readMountTable = func(path string) ([]byte, error) {
		return []byte("/dev/mapper/mapper /mnt/test ext4 rw 0 0\n"), nil
	}
	statPath = func(path string) (os.FileInfo, error) {
		if path == "/dev/mapper/mapper" {
			return mapperInfo, nil
		}
		return nil, os.ErrNotExist
	}

	req := Request{Op: OpUnmountAndClose, Mapper: "mapper", MountPoint: "/mnt/test"}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitOpError {
		t.Fatalf("expected ExitOpError, got %d", code)
	}
	if resp.OK {
		t.Fatal("expected ok=false")
	}
	if closeCalls != 0 {
		t.Fatalf("expected no close calls after unmount failure, got %d", closeCalls)
	}
}

func TestWaitForPath_Appears(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mapper")
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = os.WriteFile(path, []byte("x"), 0o600)
	}()
	if err := waitForPath(path, 200*time.Millisecond); err != nil {
		t.Fatalf("waitForPath returned error: %v", err)
	}
}

func TestWaitForPath_Timeout(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing")
	if err := waitForPath(path, 20*time.Millisecond); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestCloseMapperWithRetry(t *testing.T) {
	originalLockMapper := lockMapper
	originalDelay := closeMapperRetryDelay
	closeMapperRetryDelay = time.Nanosecond
	t.Cleanup(func() {
		lockMapper = originalLockMapper
		closeMapperRetryDelay = originalDelay
	})

	attempts := 0
	lockMapper = func(mapper string) error {
		attempts++
		if attempts < 3 {
			return errors.New("busy")
		}
		return nil
	}

	if err := closeMapperWithRetry("mapper"); err != nil {
		t.Fatal(err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestEnsureDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := ensureDirectory(dir); err != nil {
		t.Fatalf("expected existing directory to pass: %v", err)
	}
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureDirectory(file); err == nil {
		t.Fatal("expected file path to fail")
	}
}
