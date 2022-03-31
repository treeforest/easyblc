package blc

import (
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/dao"
	"math/big"
)

//BlockIterator block iterator
type BlockIterator struct {
	dao *dao.DAO
	cur []byte // 当前迭代区块的哈希
}

func NewBlockIterator(d *dao.DAO) *BlockIterator {
	return &BlockIterator{dao: d, cur: d.GetLatestBlockHash()}
}

func (it *BlockIterator) Next() (*Block, error) {
	if it.cur == nil {
		return nil, nil
	}

	var hash big.Int
	hash.SetBytes(it.cur)
	if big.NewInt(0).Cmp(&hash) == 0 {
		return nil, nil
	}

	var block Block
	data, err := it.dao.GetBlock(it.cur)
	if err != nil {
		return nil, fmt.Errorf("get block from database failed:%v", err)
	}

	err = block.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("block unmarshal failed:%v", err)
	}

	it.cur = block.PreHash[:]
	return &block, nil
}
