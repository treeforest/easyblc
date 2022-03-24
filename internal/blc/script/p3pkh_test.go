package script

import (
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"github.com/treeforest/easyblc/internal/blc/walletmgr/wallet"
	"testing"
)

func BenchmarkP2PKH(b *testing.B) {
	w := wallet.New()
	mockTx := []byte("this is a mock transaction")
	txHash := sha256.Sum256(mockTx)

	for i := 0; i < b.N; i++ {
		scriptPubKey, err := GenerateScriptPubKey(w.GetAddress())
		require.NoError(b, err)

		scriptSig, err := GenerateScriptSig(txHash[:], &w.Key)
		require.NoError(b, err)

		ok := Verify(txHash[:], scriptSig, scriptPubKey)
		require.Equal(b, true, ok)
	}
}
