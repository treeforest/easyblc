package utils

import "github.com/treeforest/easyblc/pkg/base58check"

func IsValidAddress(addr string) bool {
	_, err := base58check.Decode([]byte(addr))
	return err == nil
}
