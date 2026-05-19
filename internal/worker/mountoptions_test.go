package worker

import (
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestParseMountOptions_KnownFlags(t *testing.T) {
	flags, data := parseMountOptions([]string{"nosuid", "nodev", "noexec", "ro"})
	if flags&unix.MS_NOSUID == 0 {
		t.Error("expected MS_NOSUID")
	}
	if flags&unix.MS_NODEV == 0 {
		t.Error("expected MS_NODEV")
	}
	if flags&unix.MS_NOEXEC == 0 {
		t.Error("expected MS_NOEXEC")
	}
	if flags&unix.MS_RDONLY == 0 {
		t.Error("expected MS_RDONLY")
	}
	if data != "" {
		t.Errorf("expected empty data string, got %q", data)
	}
}

func TestParseMountOptions_UnknownGoToData(t *testing.T) {
	flags, data := parseMountOptions([]string{"uid=1000", "gid=1000", "umask=022"})
	if flags != 0 {
		t.Errorf("expected no flags, got %d", flags)
	}
	parts := strings.Split(data, ",")
	if len(parts) != 3 {
		t.Errorf("expected 3 data parts, got %v", parts)
	}
}

func TestParseMountOptions_Mixed(t *testing.T) {
	flags, data := parseMountOptions([]string{"nosuid", "nodev", "uid=1000", "noatime"})
	if flags&unix.MS_NOSUID == 0 {
		t.Error("expected MS_NOSUID")
	}
	if flags&unix.MS_NODEV == 0 {
		t.Error("expected MS_NODEV")
	}
	if flags&unix.MS_NOATIME == 0 {
		t.Error("expected MS_NOATIME")
	}
	if data != "uid=1000" {
		t.Errorf("expected data=uid=1000, got %q", data)
	}
}

func TestParseMountOptions_Empty(t *testing.T) {
	flags, data := parseMountOptions(nil)
	if flags != 0 || data != "" {
		t.Errorf("expected zero flags and empty data, got flags=%d data=%q", flags, data)
	}
}

func TestParseMountOptions_RWIsZero(t *testing.T) {
	flags, _ := parseMountOptions([]string{"rw"})
	if flags != 0 {
		t.Errorf("rw should contribute 0 to flags, got %d", flags)
	}
}
