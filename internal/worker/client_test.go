package worker

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestClientUsesNonInteractiveSudoWithoutTerminal exists so daemon-style calls
// never block on sudo prompts; it proves that by faking a non-terminal stdin
// and checking that sudo gets the -n flag.
func TestClientUsesNonInteractiveSudoWithoutTerminal(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = originalStdinIsTerminal })

	sudoPath, argsPath := fakeSudo(t, `printf '{"ok":true,"message":"test-uuid"}\n'`)
	client := &Client{selfPath: "/usr/local/bin/luks-automount", sudoPath: sudoPath}

	uuid, err := client.ReadUUID("/dev/sda")
	if err != nil {
		t.Fatal(err)
	}
	if uuid != "test-uuid" {
		t.Fatalf("got uuid %q", uuid)
	}

	got := readArgs(t, argsPath)
	want := []string{"-n", "/usr/local/bin/luks-automount", SubcommandName}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got args %v, want %v", got, want)
	}
}

// TestClientAllowsInteractiveSudoWithTerminal exists so the CLI can still ask
// sudo for credentials; it proves that by faking a terminal stdin and checking
// that sudo is called without -n.
func TestClientAllowsInteractiveSudoWithTerminal(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = originalStdinIsTerminal })

	sudoPath, argsPath := fakeSudo(t, `printf '{"ok":true,"message":"test-uuid"}\n'`)
	client := &Client{selfPath: "/usr/local/bin/luks-automount", sudoPath: sudoPath}

	if _, err := client.ReadUUID("/dev/sda"); err != nil {
		t.Fatal(err)
	}

	got := readArgs(t, argsPath)
	want := []string{"/usr/local/bin/luks-automount", SubcommandName}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got args %v, want %v", got, want)
	}
}

// TestClientReturnsCapturedSudoError exists so callers see the real sudo
// failure; it proves that by scripting stderr output and checking it is kept in
// the returned error.
func TestClientReturnsCapturedSudoError(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = originalStdinIsTerminal })

	sudoPath, _ := fakeSudo(t, `printf 'sudo: a password is required\n' >&2
exit 1`)
	client := &Client{selfPath: "/usr/local/bin/luks-automount", sudoPath: sudoPath}

	_, err := client.ReadUUID("/dev/sda")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "sudo: a password is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestClientReturnsMountPointBusyError exists so busy worker responses stay
// structured on the client side; it proves that by returning busy JSON and
// checking that UnmountAndClose yields MountPointBusyError.
func TestClientReturnsMountPointBusyError(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = originalStdinIsTerminal })

	sudoPath, _ := fakeSudo(t, `cat <<'JSON'
	{"ok":false,"code":"mount_point_busy","message":"mount point /mnt/usb is used by 1 process","mount_users":[{"pid":123,"name":"nautilus","cmdline":"nautilus"}]}
JSON
exit 1`)
	client := &Client{selfPath: "/usr/local/bin/luks-automount", sudoPath: sudoPath}

	err := client.UnmountAndClose(&Request{Mapper: "usb", MountPoint: "/mnt/usb"})
	var busy *MountPointBusyError
	if !errors.As(err, &busy) {
		t.Fatalf("got %T %v, want MountPointBusyError", err, err)
	}
	if busy.MountPoint != "/mnt/usb" || len(busy.Users) != 1 || busy.Users[0].PID != 123 {
		t.Fatalf("unexpected busy error: %+v", busy)
	}
	if !strings.Contains(err.Error(), "PID 123 nautilus") {
		t.Fatalf("busy error does not include process details: %v", err)
	}
}

// TestRunInteractiveSudoSkipsWithoutTerminal exists so daemon startup does not
// probe sudo in non-interactive runs; it proves that by removing sudo from PATH
// and checking the helper still succeeds.
func TestRunInteractiveSudoSkipsWithoutTerminal(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = originalStdinIsTerminal })
	t.Setenv("PATH", t.TempDir())

	if err := RunInteractiveSudo(); err != nil {
		t.Fatal(err)
	}
}

// fakeSudo exists to keep client tests local; it writes a sudo stub that logs
// argv and returns a scripted response.
func fakeSudo(t *testing.T, body string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	sudoPath := filepath.Join(dir, "sudo")
	argsPath := filepath.Join(dir, "args")
	content := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' \"$@\" > %q\n%s\n", argsPath, body)
	if err := os.WriteFile(sudoPath, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
	return sudoPath, argsPath
}

// readArgs exists to let tests assert exactly how fakeSudo was invoked.
func readArgs(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}
