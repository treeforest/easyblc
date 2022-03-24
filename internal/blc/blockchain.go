package blc

import (
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/dao"
	"github.com/treeforest/easyblc/pkg/base58check"
	"github.com/treeforest/easyblc/pkg/merkle"
	log "github.com/treeforest/logger"
)

type BlockChain struct {
	dao     *dao.DAO
	utxoSet []UTXO
}

func GetBlockChain() *BlockChain {
	if dao.IsNotExistDB() {
		log.Fatal("block database not exist")
	}
	d, err := dao.Load()
	if err != nil {
		log.Fatal("load block database failed: ", err)
	}
	return &BlockChain{dao: d}
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
