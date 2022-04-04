package blc

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"
)

type Transaction struct {
	Hash  [32]byte   // 交易哈希
	Vins  []TxInput  // 输入列表
	Vouts []TxOutput // 输出列表
	Time  int64      // 交易时间戳
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
		Vins:  []TxInput{*input},
		Vouts: []TxOutput{*output},
		Time:  time.Now().UnixNano(),
	}
	err = coinbase.HashTransaction()
	if err != nil {
		return nil, fmt.Errorf("caculate transaction hash failed:%v", err)
	}
	return coinbase, nil
}

// NewTransaction 普通转账交易
func NewTransaction(vins []TxInput, vouts []TxOutput) (*Transaction, error) {
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
	data, err := json.Marshal(tmp)
	if err != nil {
		return [32]byte{}, err
	}
	hash := sha256.Sum256(data)
	return hash, nil
}

func (tx *Transaction) IsCoinbase() (uint64, bool) {
	if len(tx.Vins) != 1 || len(tx.Vouts) != 1 {
		return 0, false
	}
	if tx.Vins[0].IsCoinbase() == false {
		return 0, false
	}
	return tx.Vouts[0].Value, true
}

func (tx *Transaction) Marshal() ([]byte, error) {
	return json.Marshal(tx)
}

func (tx *Transaction) Unmarshal(data []byte) error {
	return json.Unmarshal(data, tx)
}

func (tx *Transaction) String() string {
	s := fmt.Sprintf("hash:%x\n", tx.Hash)
	s += fmt.Sprintf("time:%d\n", tx.Time)
	s += "vins: \n"
	for i, vin := range tx.Vins {
		s += fmt.Sprintf("\t%d\n", i)
		if vin.IsCoinbase() {
			s += fmt.Sprintf("\t\tvout:%x\n", vin.Vout)
			s += fmt.Sprintf("\t\tcoinbaseDataSize:%d\n", vin.CoinbaseDataSize)
			s += fmt.Sprintf("\t\tconibbaseData:%s\n", vin.CoinbaseData)
			continue
		}
		s += fmt.Sprintf("\t\ttxid:%x\n", vin.TxId)
		s += fmt.Sprintf("\t\tvout:%d\n", vin.Vout)
		s += fmt.Sprintf("\t\tscriptSig:%x\n", vin.ScriptSig)
	}
	s += "vouts:\n"
	for i, vout := range tx.Vouts {
		s += fmt.Sprintf("\t%d\n", i)
		s += fmt.Sprintf("\t\tvalue:%d\n", vout.Value)
		s += fmt.Sprintf("\t\taddress:%s\n", vout.Address)
		s += fmt.Sprintf("\t\tscriptPubKey:%x\n", vout.ScriptPubKey)
	}
	return s
}
