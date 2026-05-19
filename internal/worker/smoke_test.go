//go:build integration

package worker

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const smokePassphrase = "smoke-test-passphrase-42"
const smokeMapper = "luks-smoke-test"
const smokeSetupMapper = "luks-smoke-setup"

var systemToolDirs = []string{
	"/usr/sbin", "/sbin", "/usr/bin", "/bin", "/usr/local/sbin", "/usr/local/bin",
}

func smokeTool(t *testing.T, name string) string {
	t.Helper()
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	for _, dir := range systemToolDirs {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Fatalf("required tool %q not found in PATH or common system dirs", name)
	return ""
}

func TestSmoke_WorkerProtocol(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("integration smoke test requires root")
	}
	if testing.Short() {
		t.Skip("skipping smoke test in short mode")
	}
	smokeCheckLoopDevices(t)

	loopDev := smokeCreateLUKSDevice(t)

	uuid := smokeReadUUID(t, loopDev)
	if uuid == "" {
		t.Fatal("empty UUID returned by read_uuid")
	}
	t.Logf("LUKS UUID: %s", uuid)

	mountPoint, err := os.MkdirTemp("/mnt", "luks-smoke-*")
	if err != nil {
		t.Fatalf("create mount point: %v", err)
	}
	t.Cleanup(func() { os.Remove(mountPoint) })

	smokeUnlockAndMount(t, loopDev, mountPoint)
	smokeUnmountAndClose(t, mountPoint)
}

func smokeCheckLoopDevices(t *testing.T) {
	t.Helper()
	losetup := smokeTool(t, "losetup")

	probe, err := os.CreateTemp("", "luks-loop-probe-*")
	if err != nil {
		t.Fatalf("create probe file: %v", err)
	}
	probePath := probe.Name()
	probe.Close()
	defer os.Remove(probePath)

	if err := os.Truncate(probePath, 512); err != nil {
		t.Fatalf("truncate probe file: %v", err)
	}

	out, attachErr := exec.Command(losetup, "-f", "--show", probePath).CombinedOutput()
	if attachErr != nil {
		t.Skipf("loop devices not available in this environment (%v: %s); run on bare metal or a VM", attachErr, out)
	}
	exec.Command(losetup, "-d", strings.TrimSpace(string(out))).Run()
}

func smokeCreateLUKSDevice(t *testing.T) string {
	t.Helper()

	dd := smokeTool(t, "dd")
	losetup := smokeTool(t, "losetup")
	cryptsetup := smokeTool(t, "cryptsetup")
	mkfsext4 := smokeTool(t, "mkfs.ext4")

	img, err := os.CreateTemp("", "luks-smoke-*.img")
	if err != nil {
		t.Fatal(err)
	}
	imgPath := img.Name()
	img.Close()
	t.Cleanup(func() { os.Remove(imgPath) })

	if out, err := exec.Command(dd, "if=/dev/zero", "of="+imgPath, "bs=1M", "count=32").CombinedOutput(); err != nil {
		t.Fatalf("dd: %v\n%s", err, out)
	}

	loopOut, err := exec.Command(losetup, "-f", "--show", imgPath).Output()
	if err != nil {
		t.Fatalf("losetup: %v", err)
	}
	loopDev := strings.TrimSpace(string(loopOut))
	t.Cleanup(func() { exec.Command(losetup, "-d", loopDev).Run() })

	format := exec.Command(cryptsetup, "luksFormat", "--batch-mode", "--type", "luks2", "--key-file", "-", loopDev)
	format.Stdin = strings.NewReader(smokePassphrase)
	if out, err := format.CombinedOutput(); err != nil {
		t.Fatalf("cryptsetup luksFormat: %v\n%s", err, out)
	}

	open := exec.Command(cryptsetup, "open", "--key-file", "-", loopDev, smokeSetupMapper)
	open.Stdin = strings.NewReader(smokePassphrase)
	if out, err := open.CombinedOutput(); err != nil {
		t.Fatalf("cryptsetup open (setup): %v\n%s", err, out)
	}
	defer exec.Command(cryptsetup, "close", smokeSetupMapper).Run()

	if out, err := exec.Command(mkfsext4, "-F", "/dev/mapper/"+smokeSetupMapper).CombinedOutput(); err != nil {
		t.Fatalf("mkfs.ext4: %v\n%s", err, out)
	}

	return loopDev
}

func smokeReadUUID(t *testing.T, dev string) string {
	t.Helper()
	resp, code := smokeRunServer(t, Request{Op: OpReadUUID, Dev: dev}, "")
	if code != ExitOK || !resp.OK {
		t.Fatalf("read_uuid failed (exit=%d): %s", code, resp.Message)
	}
	return resp.Message
}

func smokeUnlockAndMount(t *testing.T, dev, mountPoint string) {
	t.Helper()
	req := Request{
		Op:         OpUnlockAndMount,
		Dev:        dev,
		Mapper:     smokeMapper,
		MountPoint: mountPoint,
		FS:         "ext4",
		UID:        os.Getuid(),
		GID:        os.Getgid(),
	}
	resp, code := smokeRunServer(t, req, smokePassphrase)
	if code != ExitOK || !resp.OK {
		t.Fatalf("unlock_and_mount failed (exit=%d): %s", code, resp.Message)
	}
	t.Cleanup(func() { smokeUnmountAndClose(t, mountPoint) })
}

func smokeUnmountAndClose(t *testing.T, mountPoint string) {
	t.Helper()
	req := Request{Op: OpUnmountAndClose, Mapper: smokeMapper, MountPoint: mountPoint}
	resp, code := smokeRunServer(t, req, "")
	if code != ExitOK || !resp.OK {
		t.Logf("unmount_and_close warning (exit=%d): %s", code, resp.Message)
	}
}

func smokeRunServer(t *testing.T, req Request, passphrase string) (Response, int) {
	t.Helper()
	b, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	input := string(b) + "\n"
	if passphrase != "" {
		input += passphrase + "\n"
	}
	var out bytes.Buffer
	code := NewServer(strings.NewReader(input), &out).Run()
	var resp Response
	_ = json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp)
	return resp, code
}
