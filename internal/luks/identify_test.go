package luks

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

const testUUID = "12345678-1234-1234-1234-123456789abc"

func TestReadUUIDUsesCryptsetupLuksUUID(t *testing.T) {
	originalRunCryptsetup := runCryptsetup
	t.Cleanup(func() { runCryptsetup = originalRunCryptsetup })

	devPath := filepath.Join(t.TempDir(), "sdb1")
	if err := os.WriteFile(devPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	var got []string
	runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
		got = append([]string(nil), args...)
		return []byte(testUUID + "\n"), nil
	}

	uuid, err := ReadUUID(devPath)
	if err != nil {
		t.Fatal(err)
	}
	if uuid != testUUID {
		t.Fatalf("got uuid %q, want %q", uuid, testUUID)
	}
	want := []string{"luksUUID", devPath}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestIsLUKSUsesCryptsetupIsLuks(t *testing.T) {
	originalRunCryptsetup := runCryptsetup
	t.Cleanup(func() { runCryptsetup = originalRunCryptsetup })

	var got []string
	runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
		got = append([]string(nil), args...)
		return nil, nil
	}

	if !IsLUKS("/dev/sdb1") {
		t.Fatal("expected LUKS device")
	}
	want := []string{"isLuks", "/dev/sdb1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestReadUUIDReturnsErrNotLUKS(t *testing.T) {
	originalRunCryptsetup := runCryptsetup
	t.Cleanup(func() { runCryptsetup = originalRunCryptsetup })

	devPath := filepath.Join(t.TempDir(), "sdb1")
	if err := os.WriteFile(devPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
		return nil, errors.New("not luks")
	}

	if _, err := ReadUUID(devPath); !errors.Is(err, ErrNotLUKS) {
		t.Fatalf("got %v, want %v", err, ErrNotLUKS)
	}
}
