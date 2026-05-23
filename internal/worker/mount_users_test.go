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
	if users[0].Name != "proc-101" || users[0].Cmdline != "app-101" {
		t.Fatalf("unexpected user details: %+v", users[0])
	}
}

// TestFindMountUsersMatchesMapsWithSpaces exists so procfs map parsing does not
// lose paths under mount points with spaces; it proves that by writing a maps
// entry with spaces and checking that the PID is still reported.
func TestFindMountUsersMatchesMapsWithSpaces(t *testing.T) {
	originalProcRoot := procRoot
	procRoot = t.TempDir()
	t.Cleanup(func() { procRoot = originalProcRoot })

	makeProcProcess(t, "505", "", "", "/mnt/usb drive/lib with space.so (deleted)")

	users, err := FindMountUsers("/mnt/usb drive")
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].PID != 505 {
		t.Fatalf("unexpected users: %+v", users)
	}
}

// TestProcessCmdlineReturnsExecutableName exists so process reporting does not
// expose argv secrets; it proves that by writing a cmdline with extra args and
// checking that only the executable basename is returned.
func TestProcessCmdlineReturnsExecutableName(t *testing.T) {
	procDir := t.TempDir()
	cmdline := []byte("/usr/bin/secret-tool\x00lookup\x00password=hunter2\x00")
	if err := os.WriteFile(filepath.Join(procDir, "cmdline"), cmdline, 0o600); err != nil {
		t.Fatal(err)
	}

	if got := processCmdline(procDir); got != "secret-tool" {
		t.Fatalf("got %q, want secret-tool", got)
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
