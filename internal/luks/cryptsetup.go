package luks

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

var runCryptsetup = func(input []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("cryptsetup", args...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("cryptsetup %v: %w: %s", args, err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func isWrongPassphraseOutput(output string) bool {
	text := strings.ToLower(output)
	return strings.Contains(text, "no key available with this passphrase") || strings.Contains(text, "wrong passphrase")
}
