package wallet

import (
	"encoding/hex"
	"testing"
)

func TestWallet(t *testing.T) {
	w := New()

	t.Log(hex.EncodeToString(w.PubKey()))
}
