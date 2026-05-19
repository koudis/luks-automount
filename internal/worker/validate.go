package worker

import (
	"fmt"
	"regexp"
	"strings"

	"luks-automount/internal/config"
)

var (
	mapperRe = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	devRe    = regexp.MustCompile(`^/dev/[a-z]+$`)
)

var forbiddenOptions = map[string]bool{
	"suid": true,
	"dev":  true,
	"exec": true,
}

func validateRequest(r *Request) error {
	switch r.Op {
	case OpUnlockAndMount:
		return validateUnlockAndMount(r)
	case OpUnmountAndClose:
		return validateUnmountAndClose(r)
	case OpReadUUID:
		return validateReadUUID(r)
	default:
		return fmt.Errorf("unknown op %q", r.Op)
	}
}

func validateUnlockAndMount(r *Request) error {
	if !devRe.MatchString(r.Dev) {
		return fmt.Errorf("dev %q must match /dev/[a-z]+", r.Dev)
	}
	if !mapperRe.MatchString(r.Mapper) {
		return fmt.Errorf("mapper %q must match [a-zA-Z0-9_-]+", r.Mapper)
	}
	if err := config.ValidateMountPoint(r.MountPoint); err != nil {
		return err
	}
	if !config.IsSupportedFilesystem(r.FS) {
		return fmt.Errorf("unsupported filesystem %q", r.FS)
	}
	return nil
}

func validateUnmountAndClose(r *Request) error {
	if !mapperRe.MatchString(r.Mapper) {
		return fmt.Errorf("mapper %q must match [a-zA-Z0-9_-]+", r.Mapper)
	}
	return config.ValidateMountPoint(r.MountPoint)
}

func validateReadUUID(r *Request) error {
	if !devRe.MatchString(r.Dev) {
		return fmt.Errorf("dev %q must match /dev/[a-z]+", r.Dev)
	}
	return nil
}

func sanitizeOptions(opts string) (kept []string, dropped []string) {
	for _, raw := range strings.Split(opts, ",") {
		o := strings.TrimSpace(raw)
		if o == "" {
			continue
		}
		key := o
		if i := strings.IndexByte(o, '='); i >= 0 {
			key = o[:i]
		}
		if forbiddenOptions[key] {
			dropped = append(dropped, o)
			continue
		}
		kept = append(kept, o)
	}
	return kept, dropped
}
