package luks

import (
	"errors"
)

var ErrWrongPassphrase = errors.New("wrong passphrase")

func Unlock(devPath, mapperName string, passphrase []byte) error {
	_, err := runCryptsetup(passphrase, "--batch-mode", "open", "--type", "luks", "--key-file", "-", devPath, mapperName)
	if err != nil {
		if IsWrongPassphrase(err) {
			return ErrWrongPassphrase
		}
		return err
	}
	return nil
}

func Lock(mapperName string) error {
	_, err := runCryptsetup(nil, "luksClose", mapperName)
	return err
}

func IsWrongPassphrase(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrWrongPassphrase) || isWrongPassphraseOutput(err.Error())
}
