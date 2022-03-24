package blc

import (
	"bytes"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/dao"
	"github.com/treeforest/easyblc/internal/blc/script"
	"github.com/treeforest/easyblc/pkg/base58check"
	"github.com/treeforest/easyblc/pkg/merkle"
	"github.com/treeforest/easyblc/pkg/utils"
	log "github.com/treeforest/logger"
)

type BlockChain struct {
	dao     *dao.DAO
	utxoSet UTXOSet
	pool    *TxPool
}

func GetBlockChain() *BlockChain {
	if dao.IsNotExistDB() {
		log.Fatal("block database not exist")
	}
	d, err := dao.Load()
	if err != nil {
		log.Fatal("load block database failed: ", err)
	}
	blc := &BlockChain{dao: d, pool: NewTxPool()}

	utxoSet, err := blc.FindAllUTXOSet()
	if err != nil {
		log.Fatal("find utxo set failed:", err)
	}

	blc.utxoSet = utxoSet
	return blc
}

func CreateBlockChainWithGenesisBlock(address string) *BlockChain {
	if !dao.IsNotExistDB() {
		log.Fatal("block database already exist")
	}
	if _, err := base58check.Decode([]byte(address)); err != nil {
		log.Fatalf("%s is not a address", address)
	}
	blc := &BlockChain{dao: dao.New()}
	// todo: 奖励应该是 创币奖励+交易费
	coinbaseTx, err := NewCoinbaseTransaction(50, address)
	if err != nil {
		log.Fatal("create coinbase transaction failed:", err)
	}
	block := CreateGenesisBlock([]*Transaction{coinbaseTx})
	block.MerkleTree = &merkle.Tree{}
	blockBytes, err := block.Marshal()
	if err != nil {
		log.Fatal("block marshal failed:", err)
	}
	err = blc.dao.AddBlock(block.Hash, blockBytes)
	if err != nil {
		log.Fatal("add block to db failed: ", err)
	}
	return blc
}

func (blc *BlockChain) Close() {
	if err := blc.dao.Close(); err != nil {
		log.Fatal("close database failed: ", err)
	}
}

// GetBlockIterator 返回区块迭代器
func (blc *BlockChain) GetBlockIterator() *BlockIterator {
	return NewBlockIterator(blc.dao)
}

// Traverse 遍历区块链
func (blc *BlockChain) Traverse(fn func(block *Block)) error {
	it := blc.GetBlockIterator()
	for {
		b, err := it.Next()
		if err != nil {
			return fmt.Errorf("get next block error[%v]", err)
		}
		if b == nil {
			break
		}
		fn(b)
	}
	return nil
}

// IsValidTx 验证交易合法性，并返回矿工费
func (blc *BlockChain) IsValidTx(tx *Transaction) (uint32, bool) {
	var inputAmount, outputAmount, fee uint32

	// 验证交易哈希
	hash, err := tx.CalculateHash()
	if err != nil {
		log.Debug("calculate tx hash failed:", err)
		return 0, false
	}
	if !bytes.Equal(tx.Hash, hash) {
		log.Debug("transaction hash error")
		return 0, false
	}

	// 判断输入是否合法
	for _, vin := range tx.Vins {
		if vin.IsCoinbase() {
			continue
		}
		outputs, ok := blc.utxoSet[string(vin.TxId)]
		if !ok {
			log.Debug("not found utxo")
			return 0, false
		}
		output, ok := outputs[int(vin.Vout)]
		if !ok {
			// 交易id没有对应的utxo
			log.Debug("not found txoutput")
			return 0, false
		}
		// 是否为有效的输入脚本
		ok = script.IsValidScriptSig(vin.ScriptSig)
		if !ok {
			log.Debug("invalid scriptsig")
			return 0, false
		}
		// 验证锁定脚本与解锁脚本 P2PKH
		ok = script.Verify(vin.TxId, vin.ScriptSig, output.ScriptPubKey)
		if !ok {
			log.Debug("P2PKH verify failed")
			return 0, false
		}
		inputAmount += output.Value
	}

	for _, vout := range tx.Vouts {
		// 是否为有效的地址
		if !utils.IsValidAddress(vout.Address) {
			log.Debug("invalid output address")
			return 0, false
		}
		// 是否为有效的输出脚本
		if !script.IsValidScriptPubKey(vout.ScriptPubKey) {
			log.Debug("invalid scriptpubkey")
			return 0, false
		}

		outputAmount += vout.Value
	}

	if inputAmount < outputAmount {
		// 输入金额小于输出金额
		log.Debug("inputAmount < outputAmount")
		return 0, false
	}

	fee = inputAmount - outputAmount

	return fee, true
}
