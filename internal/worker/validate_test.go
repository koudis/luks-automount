package worker

import (
	"testing"
)

func TestValidateRequest_UnknownOp(t *testing.T) {
	err := validateRequest(&Request{Op: "bogus"})
	if err == nil {
		t.Error("expected error for unknown op")
	}
}

func TestValidateUnlockAndMount(t *testing.T) {
	valid := &Request{
		Op:         OpUnlockAndMount,
		Dev:        "/dev/sda",
		Mapper:     "my-mapper",
		MountPoint: "/mnt/usb",
		FS:         "ext4",
	}
	if err := validateRequest(valid); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}

	cases := []struct {
		name string
		mutate func(*Request)
	}{
		{"bad dev no prefix", func(r *Request) { r.Dev = "sda" }},
		{"bad dev nested path", func(r *Request) { r.Dev = "/dev/disk/by-id/id" }},
		{"bad dev has space", func(r *Request) { r.Dev = "/dev/my disk" }},
		{"bad mapper special char", func(r *Request) { r.Mapper = "my mapper" }},
		{"empty mapper", func(r *Request) { r.Mapper = "" }},
		{"mount point not under /mnt", func(r *Request) { r.MountPoint = "/tmp/usb" }},
		{"mount point is exactly /mnt", func(r *Request) { r.MountPoint = "/mnt" }},
		{"unsupported fs", func(r *Request) { r.FS = "zfs" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := *valid
			tc.mutate(&r)
			if err := validateRequest(&r); err == nil {
				t.Errorf("expected error")
			}
		})
	}
}

func TestValidateUnmountAndClose(t *testing.T) {
	valid := &Request{
		Op:         OpUnmountAndClose,
		Mapper:     "my-mapper",
		MountPoint: "/mnt/usb",
	}
	if err := validateRequest(valid); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}

	bad := *valid
	bad.Mapper = ""
	if err := validateRequest(&bad); err == nil {
		t.Error("expected error for empty mapper")
	}

	bad = *valid
	bad.MountPoint = "/tmp/usb"
	if err := validateRequest(&bad); err == nil {
		t.Error("expected error for mount point outside /mnt")
	}
}

func TestValidateReadUUID(t *testing.T) {
	valid := &Request{Op: OpReadUUID, Dev: "/dev/sdb"}
	if err := validateRequest(valid); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}

	loop := *valid
	loop.Dev = "/dev/loop0"
	if err := validateRequest(&loop); err != nil {
		t.Fatalf("loop device should be valid: %v", err)
	}

	nvme := *valid
	nvme.Dev = "/dev/nvme0n1"
	if err := validateRequest(&nvme); err != nil {
		t.Fatalf("nvme device should be valid: %v", err)
	}

	bad := *valid
	bad.Dev = "/dev/disk/by-id/id"
	if err := validateRequest(&bad); err == nil {
		t.Error("expected error for nested dev path")
	}
}

func TestSanitizeOptions(t *testing.T) {
	kept, dropped := sanitizeOptions("ro,suid,noatime,dev,exec,rw")
	if len(dropped) != 3 {
		t.Errorf("expected 3 dropped, got %d: %v", len(dropped), dropped)
	}
	if len(kept) != 3 {
		t.Errorf("expected 3 kept, got %d: %v", len(kept), kept)
	}
}

func TestSanitizeOptions_KeyValue(t *testing.T) {
	kept, dropped := sanitizeOptions("uid=1000,suid,gid=1000")
	if len(dropped) != 1 || dropped[0] != "suid" {
		t.Errorf("unexpected dropped: %v", dropped)
	}
	if len(kept) != 2 {
		t.Errorf("unexpected kept: %v", kept)
	}
}

func TestSanitizeOptions_Empty(t *testing.T) {
	kept, dropped := sanitizeOptions("")
	if len(kept) != 0 || len(dropped) != 0 {
		t.Errorf("expected empty results for empty input, got kept=%v dropped=%v", kept, dropped)
	}
}
