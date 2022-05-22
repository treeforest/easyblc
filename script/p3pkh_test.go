package script

import (
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"github.com/treeforest/easyblc/walletmgr/wallet"
	"testing"
)

func BenchmarkP2PKH(b *testing.B) {
	w := wallet.New()
	mockTx := []byte("this is a mock transaction")
	txHash := sha256.Sum256(mockTx)

	for i := 0; i < b.N; i++ {
		scriptPubKey, err := GenerateScriptPubKey(w.GetAddress())
		require.NoError(b, err)

		scriptSig, err := GenerateScriptSig(txHash, &w.Key)
		require.NoError(b, err)

		ok := Verify(txHash, scriptSig, scriptPubKey)
		require.Equal(b, true, ok)
	}
}

func TestIsValidScriptPubKey(t *testing.T) {
	w := wallet.New()
	scriptPubKey, err := GenerateScriptPubKey(w.GetAddress())
	require.NoError(t, err)
	ok := IsValidScriptPubKey(scriptPubKey)
	require.Equal(t, true, ok)
}

func TestIsValidScriptSig(t *testing.T) {
	w := wallet.New()
	txHash := sha256.Sum256([]byte("this is a mock transaction"))
	scriptSig, err := GenerateScriptSig(txHash, &w.Key)
	require.NoError(t, err)
	ok := IsValidScriptSig(scriptSig)
	require.Equal(t, true, ok)
}

func TestParsePubKeyHashFromScriptSig(t *testing.T) {
	w := wallet.New()
	txHash := sha256.Sum256([]byte("this is a mock transaction"))
	scriptSig, err := GenerateScriptSig(txHash, &w.Key)
	require.NoError(t, err)
	pubKeyHash, err := ParsePubKeyHashFromScriptSig(scriptSig)
	require.NoError(t, err)
	require.NotEmpty(t, pubKeyHash)
}

func TestParsePubKeyHashFromScriptPubKey(t *testing.T) {
	w := wallet.New()
	scriptPubKey, err := GenerateScriptPubKey(w.GetAddress())
	require.NoError(t, err)
	pubKeyHash, err := ParsePubKeyHashFromScriptPubKey(scriptPubKey)
	require.NoError(t, err)
	require.NotEmpty(t, pubKeyHash)
}
