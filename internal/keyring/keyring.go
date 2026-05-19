package keyring

import (
	"errors"

	gokeyring "github.com/zalando/go-keyring"
)

const Service = "luks-automount"

var ErrNotFound = errors.New("keyring entry not found")

func Get(name string) (string, error) {
	v, err := gokeyring.Get(Service, name)
	if err != nil {
		if errors.Is(err, gokeyring.ErrNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return v, nil
}

func Set(name, passphrase string) error {
	return gokeyring.Set(Service, name, passphrase)
}

func Delete(name string) error {
	err := gokeyring.Delete(Service, name)
	if err != nil && errors.Is(err, gokeyring.ErrNotFound) {
		return ErrNotFound
	}
	return err
}
