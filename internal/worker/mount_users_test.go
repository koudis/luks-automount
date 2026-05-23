package worker

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// TestFindMountUsers exists to prove mount-user detection does not miss common
// procfs references; it does that by building fake cwd, fd, and maps entries
// and checking that only the matching PIDs are returned.
func TestFindMountUsers(t *testing.T) {
	originalProcRoot := procRoot
	procRoot = t.TempDir()
	t.Cleanup(func() { procRoot = originalProcRoot })

	if err := os.Mkdir(filepath.Join(procRoot, "self"), 0o700); err != nil {
		t.Fatal(err)
	}
	makeProcProcess(t, "101", "/mnt/usb", "", "")
	makeProcProcess(t, "202", "", "/mnt/usb/file.txt", "")
	makeProcProcess(t, "303", "", "", "/mnt/usb/lib.so (deleted)")
	makeProcProcess(t, "404", "/mnt/usbx", "", "")

	users, err := FindMountUsers("/mnt/usb")
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int, 0, len(users))
	for _, user := range users {
		got = append(got, user.PID)
	}
	want := []int{101, 202, 303}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got PIDs %v, want %v", got, want)
	}
	if users[0].Name != "proc-101" || users[0].Cmdline != "app-101 --open /mnt/usb" {
		t.Fatalf("unexpected user details: %+v", users[0])
	}
}

// makeProcProcess exists to keep the test data small; it builds just enough of
// a fake /proc/<pid> tree for FindMountUsers to inspect.
func makeProcProcess(t *testing.T, pid, cwd, fdTarget, mapTarget string) {
	t.Helper()
	procDir := filepath.Join(procRoot, pid)
	fdDir := filepath.Join(procDir, "fd")
	if err := os.MkdirAll(fdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if cwd != "" {
		if err := os.Symlink(cwd, filepath.Join(procDir, "cwd")); err != nil {
			t.Fatal(err)
		}
	}
	if fdTarget != "" {
		if err := os.Symlink(fdTarget, filepath.Join(fdDir, "3")); err != nil {
			t.Fatal(err)
		}
	}
	if mapTarget != "" {
		line := "7f000000-7f001000 r--p 00000000 08:01 1 " + mapTarget + "\n"
		if err := os.WriteFile(filepath.Join(procDir, "maps"), []byte(line), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(procDir, "comm"), []byte("proc-"+pid+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmdline := []byte("app-" + pid + "\x00--open\x00/mnt/usb\x00")
	if err := os.WriteFile(filepath.Join(procDir, "cmdline"), cmdline, 0o600); err != nil {
		t.Fatal(err)
	}
}
