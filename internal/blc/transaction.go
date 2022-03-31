package blc

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"time"

	gob "github.com/treeforest/easyblc/pkg/gob"
)

type Transaction struct {
	Hash  [32]byte    // 交易哈希
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
		Hash:  [32]byte{},
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
		Hash:  [32]byte{},
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
func (tx *Transaction) CalculateHash() ([32]byte, error) {
	tmp := *tx
	tmp.Hash = [32]byte{}
	data, err := gob.Encode(&tmp)
	if err != nil {
		return [32]byte{}, err
	}
	hash := sha256.Sum256(data)
	return hash, nil
}

func (tx *Transaction) Marshal() ([]byte, error) {
	return gob.Encode(tx)
}

func (tx *Transaction) Unmarshal(data []byte) error {
	return gob.Decode(data, tx)
}

type TransactionSlice []*Transaction

func (x TransactionSlice) Len() int           { return len(x) }
func (x TransactionSlice) Less(i, j int) bool { return bytes.Compare(x[i].Hash[:], x[j].Hash[:]) == -1 }
func (x TransactionSlice) Swap(i, j int)      { x[i], x[j] = x[j], x[i] }
