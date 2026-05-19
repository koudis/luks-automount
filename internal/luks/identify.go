package luks

import (
	"errors"
	"os"

	anatolluks "github.com/anatol/luks.go"
)

var ErrNotLUKS = errors.New("not a LUKS device")

func ReadUUID(devPath string) (string, error) {
	if _, err := os.Stat(devPath); err != nil {
		return "", err
	}
	dev, err := anatolluks.Open(devPath)
	if err != nil {
		return "", ErrNotLUKS
	}
	defer dev.Close()
	return dev.UUID(), nil
}

func IsLUKS(devPath string) bool {
	dev, err := anatolluks.Open(devPath)
	if err != nil {
		return false
	}
	dev.Close()
	return true
}
