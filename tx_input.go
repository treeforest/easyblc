package blc

import (
	"crypto/ecdsa"
	"github.com/pkg/errors"
	"github.com/treeforest/easyblc/script"
)

type TxInput struct {
	TxId             [32]byte // 引用的上一笔交易的交易哈希，创币交易初始化全为0
	Vout             uint32   // 引用的上一笔交易的输出索引，创币交易初始化为0xFFFFFFFF
	ScriptSig        []byte   // 解锁脚本
	CoinbaseDataSize int      // 创币交易长度
	CoinbaseData     []byte   // 创币交易（用户可以在这里写下任何东西，可辅助挖矿）
}

func NewTxInput(txId [32]byte, vout uint32) (*TxInput, error) {
	txInput := &TxInput{
		TxId:             txId,
		Vout:             vout,
		ScriptSig:        nil,
		CoinbaseDataSize: 0,
		CoinbaseData:     nil,
	}
	return txInput, nil
}

func NewCoinbaseTxInput(coinbaseData []byte) *TxInput {
	return &TxInput{
		TxId:             [32]byte{},
		Vout:             0xFFFFFFFF,
		ScriptSig:        nil,
		CoinbaseDataSize: len(coinbaseData),
		CoinbaseData:     coinbaseData,
	}
}

func (input *TxInput) IsCoinbase() bool {
	return input.Vout == 0xFFFFFFFF
}

// SetScriptSig 设置输入脚本
func (input *TxInput) SetScriptSig(hash [32]byte, key *ecdsa.PrivateKey) error {
	scriptSig, err := script.GenerateScriptSig(hash, key)
	if err != nil {
		return errors.WithStack(err)
	}
	input.ScriptSig = scriptSig
	return nil
}
