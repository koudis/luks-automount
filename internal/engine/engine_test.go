package engine

import (
	"errors"
	"testing"
	"time"

	"luks-automount/internal/config"
	"luks-automount/internal/monitor"
	"luks-automount/internal/worker"
)

const testUUID = "12345678-1234-1234-1234-123456789abc"

func TestReadUUIDWithRetry(t *testing.T) {
	originalReadUUID := readUUID
	originalDelay := readUUIDRetryDelay
	readUUIDRetryDelay = time.Nanosecond
	t.Cleanup(func() {
		readUUID = originalReadUUID
		readUUIDRetryDelay = originalDelay
	})

	attempts := 0
	readUUID = func(client *worker.Client, devPath string) (string, error) {
		attempts++
		if attempts < 3 {
			return "", errors.New("not ready")
		}
		return testUUID, nil
	}

	uuid, err := readUUIDWithRetry(nil, "/dev/sdb")
	if err != nil {
		t.Fatal(err)
	}
	if uuid != testUUID {
		t.Fatalf("unexpected uuid: %s", uuid)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestHandleRemoveKeepsStaleMapperStateOnCloseFailure(t *testing.T) {
	originalUnmountAndClose := unmountAndClose
	t.Cleanup(func() { unmountAndClose = originalUnmountAndClose })

	calls := 0
	unmountAndClose = func(client *worker.Client, req *worker.Request) error {
		calls++
		return errors.New("busy")
	}

	e := New(&config.Config{Disks: []config.Disk{{
		Name:           "usb",
		LUKSUUID:       testUUID,
		MapperName:     "usb",
		MountPoint:     "/mnt/usb",
		FilesystemType: "ext4",
	}}}, nil)
	state := e.stateFor(testUUID)
	state.DevPath = "/dev/sdb"
	state.Unlocked = true
	state.Mounted = true
	e.rememberDev("/dev/sdb", testUUID)

	e.handleRemove(monitor.Event{Action: monitor.ActionRemove, DevPath: "/dev/sdb"})
	if calls != 1 {
		t.Fatalf("expected one close attempt, got %d", calls)
	}
	if !state.Unlocked || state.Mounted || state.DevPath != "" {
		t.Fatalf("unexpected state after failed close: unlocked=%t mounted=%t dev=%q", state.Unlocked, state.Mounted, state.DevPath)
	}

	e.rememberDev("/dev/sdb", testUUID)
	e.handleRemove(monitor.Event{Action: monitor.ActionRemove, DevPath: "/dev/sdb"})
	if calls != 1 {
		t.Fatalf("expected duplicate remove to be ignored, got %d close attempts", calls)
	}
}
