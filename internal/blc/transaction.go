package blc

import (
	"crypto/sha256"
	"fmt"
	"time"

	gob "github.com/treeforest/easyblc/pkg/gob"
)

type Transaction struct {
	Hash  []byte      // 交易哈希
	Vins  []*TxInput  // 输入列表
	Vouts []*TxOutput // 输出列表
	Time  int64       // 交易时间戳
}

// NewCoinbaseTransaction coinbase 交易
func NewCoinbaseTransaction(reward uint64, address string, coinbaseData []byte) (*Transaction, error) {
	input := NewCoinbaseTxInput(coinbaseData)
	output, err := NewTxOutput(reward, address)
	if err != nil {
		return nil, fmt.Errorf("create coinbase transaction output error:%v", err)
	}
	coinbase := &Transaction{
		Hash:  nil,
		Vins:  []*TxInput{input},
		Vouts: []*TxOutput{output},
		Time:  time.Now().UnixNano(),
	}
	err = coinbase.HashTransaction()
	if err != nil {
		return nil, fmt.Errorf("caculate transaction hash failed:%v", err)
	}
	return coinbase, nil
}

// NewTransaction 普通转账交易
func NewTransaction(vins []*TxInput, vouts []*TxOutput) (*Transaction, error) {
	tx := &Transaction{
		Hash:  nil,
		Vins:  vins,
		Vouts: vouts,
		Time:  time.Now().UnixNano(),
	}
	if err := tx.HashTransaction(); err != nil {
		return nil, fmt.Errorf("caculate transaction hash failed:%v", err)
	}
	return tx, nil
}

// HashTransaction 生成交易哈希
func (tx *Transaction) HashTransaction() error {
	hash, err := tx.CalculateHash()
	if err != nil {
		return err
	}
	tx.Hash = hash
	return nil
}

// CalculateHash 计算区块哈希
func (tx *Transaction) CalculateHash() ([]byte, error) {
	tmp := *tx
	tmp.Hash = nil
	data, err := gob.Encode(&tmp)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	return hash[:], nil
}

//// VerifyTransaction 验证交易合法性
//func (chain *BlockChain) VerifyTransaction(target *Transaction) {
//	chain.Traverse(func(block *Block) {
//		//for _, tx := range block.Transactions {
//		//
//		//}
//	})
//}
//
//func (chain *BlockChain) VerifyTxInput(in *TxInput) bool {
//	if in.IsCoinbase() {
//		if in.ScriptSig != nil {
//			log.Warn("coinbase script sig is not nil")
//			return false
//		}
//		bi := big.NewInt(0)
//		bi.SetBytes(in.TxId)
//		if big.NewInt(0).Cmp(bi) == 0 {
//			log.Warn("coinbase txid is not zero")
//			return false
//		}
//		return true
//	}
//
//	// simple transaction
//	chain.Traverse(func(block *Block) {
//
//	})
//
//	return true
//}
