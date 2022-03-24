package blc

import (
	"bytes"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/script"
	"github.com/treeforest/easyblc/pkg/base58check"
)

type UTXOSet map[string]map[int]*TxOutput // TxId => Index => TxOut

type UTXO struct {
	TxId  []byte    // UTXO所在交易的交易哈希
	Index int       // UTXO在所属交易输出中的索引
	TxOut *TxOutput // 交易输出本身
}

// FindAllUTXOSet 获取所有的utxo
func (blc *BlockChain) FindAllUTXOSet() (UTXOSet, error) {
	utxoSet := make(UTXOSet)
	spent := make(map[string]map[uint32]struct{})
	err := blc.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for _, vin := range tx.Vins {
				if _, ok := spent[string(vin.TxId)]; !ok {
					spent[string(vin.TxId)] = map[uint32]struct{}{vin.Vout: {}}
					continue
				}
				spent[string(vin.TxId)][vin.Vout] = struct{}{}
			}
			for i, out := range tx.Vouts {
				_, exist := spent[string(tx.Hash)][uint32(i)]
				if exist {
					delete(spent[string(tx.Hash)], uint32(i))
					continue
				}
				if _, ok := utxoSet[string(tx.Hash)]; !ok {
					utxoSet[string(tx.Hash)] = map[int]*TxOutput{i: out}
					continue
				}
				utxoSet[string(tx.Hash)][i] = out
			}
		}
	})
	if err != nil {
		return nil, err
	}
	return utxoSet, nil
}

func (blc *BlockChain) GetBalance(address string) (uint32, error) {
	utxoSet, err := blc.GetUTXOSet(address)
	if err != nil {
		return 0, err
	}
	var amount uint32 = 0
	for _, outputs := range utxoSet {
		for _, output := range outputs {
			amount += output.Value
		}
	}
	return amount, nil
}

// GetUTXOSet 获取UTXO集合
func (blc *BlockChain) GetUTXOSet(address string) (UTXOSet, error) {
	spent, err := blc.GetSpentOutput(address)
	if err != nil {
		return nil, err
	}

	utxoSet := make(UTXOSet)
	err = blc.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for i, vout := range tx.Vouts {

				if address != vout.Address {
					continue
				}

				out, ok := spent[string(tx.Hash)]
				if ok {
					_, ok = out[uint32(i)]
					if ok {
						// transaction output was already spend
						continue
					}
				}

				// find unspent transaction output
				if _, ok = utxoSet[string(tx.Hash)]; !ok {
					utxoSet[string(tx.Hash)] = map[int]*TxOutput{i: vout}
				}
				utxoSet[string(tx.Hash)][i] = vout
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse block error:%v", err)
	}
	return utxoSet, nil
}

// GetSpentOutput 当前地址address已花费的输出
func (blc *BlockChain) GetSpentOutput(address string) (map[string]map[uint32]struct{}, error) {
	hash160, err := base58check.Decode([]byte(address))
	if err != nil {
		return nil, fmt.Errorf("address format error: %v", err)
	}
	spent := make(map[string]map[uint32]struct{})
	err = blc.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for _, vin := range tx.Vins {
				if vin.IsCoinbase() {
					// coinbase transaction
					continue
				}
				pubKeyHash := script.ParsePubKeyHashFromScriptSig(vin.ScriptSig)
				if bytes.Equal(hash160, pubKeyHash) {
					spent[string(vin.TxId)][vin.Vout] = struct{}{}
				}
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse block error:%v", err)
	}
	return spent, nil
}
