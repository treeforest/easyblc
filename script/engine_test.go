package script

import (
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"github.com/treeforest/easyblc/internal/blc/walletmgr/wallet"
	"testing"
)

func BenchmarkEngine_Run(b *testing.B) {
	w := wallet.New()
	mockTx := []byte("this is a mock transaction")

	for i := 0; i < b.N; i++ {
		sig, err := w.Sign(mockTx)
		require.NoError(b, err)

		output := []Op{
			{Code: CHECKSIG, Data: nil},
			{Code: EQUALVERIFY, Data: nil},
			{Code: PUSH, Data: w.PubKeyHash160()},
			{Code: HASH160, Data: nil},
			{Code: DUP, Data: nil},
		}
		input := []Op{
			{Code: PUSH, Data: w.PubKey()},
			{Code: PUSH, Data: sig},
		}

		txHash := sha256.Sum256(mockTx)
		e := Engine{Ops: make([]Op, 0), TxHash: txHash[:]}
		e.push(output)
		e.push(input)
		require.Equal(b, e.Run(), true)
	}
}
