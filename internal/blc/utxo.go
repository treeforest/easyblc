package blc

import (
	"fmt"
	log "github.com/treeforest/logger"
)

type UTXOSet map[string]map[int]*TxOutput // TxId => Index => TxOut

type UTXO struct {
	TxId  []byte    // UTXO所在交易的交易哈希
	Index int       // UTXO在所属交易输出中的索引
	TxOut *TxOutput // 交易输出本身
}

// FindAllUTXOSet 获取所有的utxo
func (chain *BlockChain) FindAllUTXOSet() (UTXOSet, error) {
	utxoSet := make(UTXOSet)
	spent := make(map[string]map[uint32]struct{})
	log.Debug("[utxo set]")
	err := chain.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			// 交易输入
			for _, vin := range tx.Vins {
				if vin.IsCoinbase() {
					continue
				}
				if _, ok := spent[string(vin.TxId)]; !ok {
					spent[string(vin.TxId)] = make(map[uint32]struct{})
				}
				spent[string(vin.TxId)][vin.Vout] = struct{}{}
			}
			// 交易输出
			for i, out := range tx.Vouts {
				if v, ok := spent[string(tx.Hash)]; ok {
					if _, ok = v[uint32(i)]; ok {
						// 被花费
						delete(spent[string(tx.Hash)], uint32(i))
						continue
					}
				}
				log.Debugf("txid:%x\tindex:%d\taddress:%s\tvalue:%d", tx.Hash, i, out.Address, out.Value)
				if _, ok := utxoSet[string(tx.Hash)]; !ok {
					utxoSet[string(tx.Hash)] = make(map[int]*TxOutput)
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

func (chain *BlockChain) GetBalance(address string) (uint64, error) {
	utxoSet, err := chain.GetUTXOSet(address)
	if err != nil {
		return 0, err
	}
	var amount uint64 = 0
	for _, outputs := range utxoSet {
		for _, output := range outputs {
			amount += output.Value
		}
	}
	return amount, nil
}

// GetUTXOSet 获取UTXO集合
func (chain *BlockChain) GetUTXOSet(address string) (UTXOSet, error) {
	utxoSet := make(UTXOSet)
	err := chain.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for i, vout := range tx.Vouts {
				if address != vout.Address {
					continue
				}
				// find unspent transaction output
				if _, ok := utxoSet[string(tx.Hash)]; !ok {
					utxoSet[string(tx.Hash)] = make(map[int]*TxOutput)
				}
				utxoSet[string(tx.Hash)][i] = vout
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse block error:%v", err)
	}

	// remove spent utxo
	err = chain.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for _, vin := range tx.Vins {
				if _, ok := utxoSet[string(vin.TxId)]; !ok {
					continue
				}
				delete(utxoSet[string(vin.TxId)], int(vin.Vout))
				if len(utxoSet[string(vin.TxId)]) == 0 {
					delete(utxoSet, string(vin.TxId))
				}
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse block error:%v", err)
	}

	return utxoSet, nil
}
