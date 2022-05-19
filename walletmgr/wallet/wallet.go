package wallet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"github.com/treeforest/easyblc/base58check"

	"golang.org/x/crypto/ripemd160"
)

// Wallet 钱包
type Wallet struct {
	Key ecdsa.PrivateKey // 私钥
	Pub []byte           // 公钥
}

func New() *Wallet {
	key, pub := newEcdsaKeyPair()
	return &Wallet{Key: key, Pub: pub}
}

func newEcdsaKeyPair() (ecdsa.PrivateKey, []byte) {
	curve := elliptic.P256()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		panic(err)
	}
	pub := elliptic.Marshal(curve, key.PublicKey.X, key.PublicKey.Y)
	return *key, pub
}

// GetAddress 钱包地址
func (w *Wallet) GetAddress() []byte {
	hash160 := w.PubKeyHash160()
	address := base58check.Encode(hash160)
	return address
}

func (w *Wallet) Sign(data []byte) ([]byte, error) {
	hash := sha256.Sum256(data)
	return ecdsa.SignASN1(rand.Reader, &w.Key, hash[:])
}

func (w *Wallet) PubKeyHash160() []byte {
	hash := sha256.Sum256(w.Pub)
	r := ripemd160.New()
	r.Write(hash[:])
	hash160 := r.Sum(nil)
	return hash160[:]
}

func (w *Wallet) PubKey() []byte {
	pub := make([]byte, len(w.Pub))
	copy(pub, w.Pub)
	return pub
}
