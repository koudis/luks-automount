package luks

import (
	"errors"
	"os"
	"strings"
)

var ErrNotLUKS = errors.New("not a LUKS device")

func ReadUUID(devPath string) (string, error) {
	if _, err := os.Stat(devPath); err != nil {
		return "", err
	}
	out, err := runCryptsetup(nil, "luksUUID", devPath)
	if err != nil {
		return "", ErrNotLUKS
	}
	uuid := strings.TrimSpace(string(out))
	if uuid == "" {
		return "", ErrNotLUKS
	}
	return uuid, nil
}

func IsLUKS(devPath string) bool {
	_, err := runCryptsetup(nil, "isLuks", devPath)
	return err == nil
}
