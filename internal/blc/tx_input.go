package blc

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/script"
)

type TxInput struct {
	TxId             []byte // 引用的上一笔交易的交易哈希，创币交易初始化全为0
	Vout             uint32 // 引用的上一笔交易的输出索引，创币交易初始化为0xFFFFFFFF
	ScriptSig        []byte // 解锁脚本
	CoinbaseDataSize int    // 创币交易长度
	CoinbaseData     []byte // 创币交易（用户可以在这里写下任何东西，可辅助挖矿）
}

func NewTxInput(txId []byte, vout uint32, key *ecdsa.PrivateKey) (*TxInput, error) {
	scriptSig, err := script.GenerateScriptSig(txId, key)
	if err != nil {
		return nil, fmt.Errorf("generate script sig failed:%v", err)
	}
	txInput := &TxInput{
		TxId:             txId,
		Vout:             vout,
		ScriptSig:        scriptSig,
		CoinbaseDataSize: 0,
		CoinbaseData:     nil,
	}
	return txInput, nil
}

func NewCoinbaseTxInput(coinbaseData []byte) *TxInput {
	zeroTxId := [32]byte{}
	return &TxInput{
		TxId:             zeroTxId[:],
		Vout:             0xFFFFFFFF,
		ScriptSig:        nil,
		CoinbaseDataSize: len(coinbaseData),
		CoinbaseData:     coinbaseData,
	}
}

func (input *TxInput) IsCoinbase() bool {
	return input.Vout == 0xFFFFFFFF
}
