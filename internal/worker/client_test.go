package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

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

func TestRunInteractiveSudoSkipsWithoutTerminal(t *testing.T) {
	originalStdinIsTerminal := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = originalStdinIsTerminal })
	t.Setenv("PATH", t.TempDir())

	if err := RunInteractiveSudo(); err != nil {
		t.Fatal(err)
	}
}

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

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(b)), "\n")
}
