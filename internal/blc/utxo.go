package blc

import (
	"bytes"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/script"
	"github.com/treeforest/easyblc/pkg/base58check"
)

type UTXO struct {
	TxId  []byte    // UTXO所在交易的交易哈希
	Index int       // UTXO在所属交易输出中的索引
	TxOut *TxOutput // 交易输出本身
}

// GetAllUTXOSet 获取所有的utxo
func (blc *BlockChain) GetAllUTXOSet() []UTXO {
	//utxoSet := make(map[string][]UTXO)
	//blc.Traverse(func(block *Block) {
	//	for _, tx := range block.Transactions {
	//		//for _, out := range tx.Vouts {
	//		//	if _, ok := utxoSet[out.]
	//		//}
	//		//for _, vin := range tx.Vins {
	//		//
	//		//}
	//	}
	//})
	return nil
}

func (blc *BlockChain) GetBalance(address string) (uint32, error) {
	utxoSet, err := blc.GetUTXOSet(address)
	if err != nil {
		return 0, err
	}
	var amount uint32 = 0
	for _, utxo := range utxoSet {
		amount += utxo.TxOut.Value
	}
	return amount, nil
}

// GetUTXOSet 获取UTXO集合
func (blc *BlockChain) GetUTXOSet(address string) ([]UTXO, error) {
	spent, err := blc.SpentOutput(address)
	if err != nil {
		return nil, err
	}
	utxoSet := make([]UTXO, 0)
	err = blc.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for i, vout := range tx.Vouts {
				if address == vout.Address {
					if v, ok := spent[string(tx.Hash)]; ok {
						var j uint32
						for _, j = range v {
							if uint32(i) == j {
								// 已花费的utxo
								break
							}
						}
						if j == uint32(len(v)) {
							// 没有属于address的交易输出
							continue
						}
					}
					// 找到未花费的utxo
					utxo := UTXO{TxId: tx.Hash, Index: i, TxOut: vout}
					utxoSet = append(utxoSet, utxo)
				}
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse block error:%v", err)
	}
	return utxoSet, nil
}

// SpentOutput 当前地址address已花费的输出
func (blc *BlockChain) SpentOutput(address string) (map[string][]uint32, error) {
	hash160, err := base58check.Decode([]byte(address))
	if err != nil {
		return nil, fmt.Errorf("address format error: %v", err)
	}
	var spent map[string][]uint32
	err = blc.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			for _, vin := range tx.Vins {
				if vin.IsCoinbase() {
					// coinbase transaction
					continue
				}
				pubKeyHash := script.ParsePubKeyHashFromScriptSig(vin.ScriptSig)
				if bytes.Equal(hash160, pubKeyHash) {
					txHash := string(vin.TxId) // txInput对应的txOutput的哈希
					if _, ok := spent[txHash]; ok {
						spent[txHash] = append(spent[txHash], vin.Vout)
					} else {
						spent[txHash] = []uint32{vin.Vout}
					}
				}
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse block error:%v", err)
	}
	return spent, nil
}
