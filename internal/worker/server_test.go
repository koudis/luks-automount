package worker

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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
	req := Request{Op: OpReadUUID, Dev: "/dev/sda1"}
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
		Dev:        "/dev/sda1",
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
