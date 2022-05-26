package script

import (
	"crypto/sha256"
	"github.com/stretchr/testify/require"
	"github.com/treeforest/easyblc/walletmgr/wallet"
	"testing"
)

func BenchmarkEngine_Run(b *testing.B) {
	// 所有者
	owner := wallet.New()

	// 模拟的交易
	mockTx := []byte("this is a mock transaction")

	for i := 0; i < b.N; i++ {
		sig, err := owner.Sign(mockTx)
		require.NoError(b, err)

		// 上一笔交易的交易输出
		output := []Op{
			{Code: CHECKSIG, Data: nil},
			{Code: EQUALVERIFY, Data: nil},
			{Code: PUSH, Data: owner.PubKeyHash160()},
			{Code: HASH160, Data: nil},
			{Code: DUP, Data: nil},
		}

		// 当前交易的交易输入
		input := []Op{
			{Code: PUSH, Data: owner.PubKey()},
			{Code: PUSH, Data: sig},
		}

		txHash := sha256.Sum256(mockTx)
		e := Engine{Ops: make([]Op, 0), TxHash: txHash[:]}
		e.push(output)
		e.push(input)
		require.Equal(b, e.Run(), true)
	}
}
