package luks

import (
	"errors"
	"reflect"
	"testing"
)

func TestUnlockUsesCryptsetupOpen(t *testing.T) {
	originalRunCryptsetup := runCryptsetup
	t.Cleanup(func() { runCryptsetup = originalRunCryptsetup })

	var gotInput []byte
	var gotArgs []string
	runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
		gotInput = append([]byte(nil), input...)
		gotArgs = append([]string(nil), args...)
		return nil, nil
	}

	if err := Unlock("/dev/sdb1", "samsung-data", []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if string(gotInput) != "secret" {
		t.Fatalf("got input %q", gotInput)
	}
	wantArgs := []string{"--batch-mode", "open", "--type", "luks", "--key-file", "-", "/dev/sdb1", "samsung-data"}
	if !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("got %v, want %v", gotArgs, wantArgs)
	}
}

func TestUnlockDetectsWrongPassphrase(t *testing.T) {
	originalRunCryptsetup := runCryptsetup
	t.Cleanup(func() { runCryptsetup = originalRunCryptsetup })

	runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
		return nil, errors.New("No key available with this passphrase")
	}

	if err := Unlock("/dev/sdb1", "samsung-data", []byte("bad")); !IsWrongPassphrase(err) {
		t.Fatalf("expected wrong passphrase error, got %v", err)
	}
}

func TestLockUsesCryptsetupLuksClose(t *testing.T) {
	originalRunCryptsetup := runCryptsetup
	t.Cleanup(func() { runCryptsetup = originalRunCryptsetup })

	var got []string
	runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
		got = append([]string(nil), args...)
		return nil, nil
	}

	if err := Lock("samsung-data"); err != nil {
		t.Fatal(err)
	}
	want := []string{"luksClose", "samsung-data"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
