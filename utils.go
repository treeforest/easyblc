package blc

import (
	"errors"
	"github.com/treeforest/easyblc/base58check"
)

func IsValidAddress(addr string) bool {
	_, err := base58check.Decode([]byte(addr))
	return err == nil
}

func BytesToByte32(b []byte) ([32]byte, error) {
	if len(b) != 32 {
		return [32]byte{}, errors.New("length not equal 32")
	}
	d := [32]byte{}
	for i := 0; i < 32; i++ {
		d[i] = b[i]
	}
	return d, nil
}
