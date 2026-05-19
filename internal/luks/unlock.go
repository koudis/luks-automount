package luks

import (
	"fmt"

	anatolluks "github.com/anatol/luks.go"
)

func Unlock(devPath, mapperName string, passphrase []byte) error {
	dev, err := anatolluks.Open(devPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", devPath, err)
	}
	defer dev.Close()
	if err := dev.UnlockAny(passphrase, mapperName); err != nil {
		return err
	}
	return nil
}

func Lock(mapperName string) error {
	return anatolluks.Lock(mapperName)
}

func IsWrongPassphrase(err error) bool {
	if err == nil {
		return false
	}
	return err == anatolluks.ErrPassphraseDoesNotMatch
}
