package worker

import (
	"bytes"
	"encoding/json"
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
	req := Request{
		Op:         OpUnmountAndClose,
		Mapper:     "no-such-mapper",
		MountPoint: "/mnt/no-such-path",
	}
	b, _ := json.Marshal(req)
	resp, code := runServer(string(b) + "\n")
	if code != ExitOpError {
		t.Errorf("expected ExitOpError, got %d", code)
	}
	if resp.OK {
		t.Error("expected ok=false")
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
