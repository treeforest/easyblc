package blc

import (
	"bytes"
	"errors"
	"sync"
)

// TxPool 交易池
type TxPool struct {
	sync.Mutex                           // locker
	cache      map[uint32][]*Transaction // fee => txs
}

func NewTxPool() *TxPool {
	return &TxPool{cache: map[uint32][]*Transaction{}}
}

// PutTxToPool 将交易放入内存池
func (blc *BlockChain) PutTxToPool(tx *Transaction) error {
	fee, ok := blc.IsValidTx(tx)
	if !ok {
		return errors.New("invalid transaction")
	}
	blc.pool.Put(fee, tx)
	return nil
}

// Put 将交易存入交易池
func (pool *TxPool) Put(fee uint32, tx *Transaction) {
	pool.Lock()
	defer pool.Unlock()
	if _, ok := pool.cache[fee]; !ok {
		pool.cache[fee] = []*Transaction{tx}
		return
	}
	for _, v := range pool.cache[fee] {
		if bytes.Equal(v.Hash, tx.Hash) {
			// transaction already exist in pool
			return
		}
	}
	pool.cache[fee] = append(pool.cache[fee], tx)
}

func (pool *TxPool) Get() {

}

func (pool *TxPool) Remove() {

}
