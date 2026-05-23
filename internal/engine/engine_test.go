package engine

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"luks-automount/internal/config"
	"luks-automount/internal/monitor"
	"luks-automount/internal/worker"
)

const testUUID = "12345678-1234-1234-1234-123456789abc"

// TestReadUUIDWithRetry exists so short add-event races do not lose a disk; it
// proves that by failing the first two UUID reads and succeeding on the third.
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

// TestHandleRemoveShowsBusyDialog exists so daemon auto-lock remains visible to
// the user; it proves that by returning a busy worker error and checking that
// the dialog hook receives the same mount-user list.
func TestHandleRemoveShowsBusyDialog(t *testing.T) {
	originalUnmountAndClose := unmountAndClose
	originalShow := showMountPointBusy
	t.Cleanup(func() {
		unmountAndClose = originalUnmountAndClose
		showMountPointBusy = originalShow
	})

	users := []worker.MountUser{{PID: 123, Name: "terminal"}}
	unmountAndClose = func(client *worker.Client, req *worker.Request) error {
		return &worker.MountPointBusyError{MountPoint: req.MountPoint, Message: "busy", Users: users}
	}
	showCalls := 0
	showMountPointBusy = func(mountPoint string, gotUsers []worker.MountUser) error {
		showCalls++
		if mountPoint != "/mnt/usb" || !reflect.DeepEqual(gotUsers, users) {
			t.Fatalf("unexpected dialog input: %s %+v", mountPoint, gotUsers)
		}
		return nil
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
	if showCalls != 1 {
		t.Fatalf("expected one dialog call, got %d", showCalls)
	}
}

// TestHandleRemoveKeepsStaleMapperStateOnCloseFailure exists so a failed remove
// pass does not trigger duplicate close attempts; it proves that by running two
// remove events and checking that the close path is entered only once.
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
