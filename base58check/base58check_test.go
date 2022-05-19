package base58check

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ripemd160"
	"testing"
)

func TestBase58check(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	pub := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)

	hash := sha256.Sum256(pub)
	r := ripemd160.New()
	r.Write(hash[:])
	hash160 := r.Sum(nil)

	address := Encode(hash160)
	// t.Log(string(address))

	dexHash160, err := Decode(address)
	require.NoError(t, err)
	require.Equal(t, hash160, dexHash160)
}
