package blc

import (
	"bytes"
	"errors"
	"sync"
)

// TxPool 交易池
type TxPool struct {
	sync.Mutex                           // locker
	fees       *UniquePriorityQueue      // sorted fees,priority queue
	cache      map[uint64][]*TxPoolEntry // fee => txs
}

type TxPoolEntry struct {
	Fee uint64
	Tx  *Transaction
}

func NewTxPool() *TxPool {
	return &TxPool{
		fees:  NewUniquePriorityQueue(),
		cache: map[uint64][]*TxPoolEntry{},
	}
}

// AddToTxPool 将交易放入内存池
func (chain *BlockChain) AddToTxPool(tx *Transaction) error {
	fee, ok := chain.IsValidTx(tx)
	if !ok {
		return errors.New("invalid transaction")
	}
	chain.txPool.Put(fee, &TxPoolEntry{Fee: fee, Tx: tx})
	return nil
}

// GetTxsFromTxPool 从交易池中获取交易
func (chain *BlockChain) GetTxsFromTxPool(count int) []*TxPoolEntry {
	if count <= 0 {
		return []*TxPoolEntry{}
	}
	if count > 256 {
		count = 256 // 一次最多只能获取256个交易
	}
	return chain.txPool.Get(count)
}

// Put 将交易存入交易池
func (pool *TxPool) Put(fee uint64, entry *TxPoolEntry) {
	pool.Lock()
	defer pool.Unlock()

	pool.fees.Push(fee)

	if pool.cache == nil {
		pool.cache = make(map[uint64][]*TxPoolEntry)
	}

	if _, ok := pool.cache[fee]; !ok {
		pool.cache[fee] = []*TxPoolEntry{entry}
		return
	}

	for _, v := range pool.cache[fee] {
		if bytes.Equal(v.Tx.Hash, entry.Tx.Hash) {
			// transaction already exist in txPool
			return
		}
	}

	pool.cache[fee] = append(pool.cache[fee], entry)
}

// Get 从交易池中获取交易
// 参数：
//		count: 获取多少个交易，将返回小于等于交易的个数。
func (pool *TxPool) Get(count int) []*TxPoolEntry {
	pool.Lock()
	defer pool.Unlock()

	var (
		counter = 0
		i       int
		key     uint64
	)
	txs := make([]*TxPoolEntry, 0, count)

	for !pool.Empty() {
		key = pool.fees.Front()

		for i = 0; i < len(pool.cache[key]); i++ {
			txs = append(txs, pool.cache[key][i])
			counter++
			if counter == count {
				pool.cache[key] = pool.cache[key][i+1:] // remove used
				return txs
			}
		}

		delete(pool.cache, key)
		pool.fees.Pop() // dequeue
	}

	return txs
}

func (pool *TxPool) Empty() bool {
	return len(pool.cache) == 0 && pool.fees.Empty()
}

// UniquePriorityQueue 唯一元素的优先级队列, 按照从大到小排列
type UniquePriorityQueue struct {
	queue []uint64
}

func NewUniquePriorityQueue() *UniquePriorityQueue {
	return &UniquePriorityQueue{queue: make([]uint64, 0)}
}

func (u *UniquePriorityQueue) Push(item uint64) {
	if u.queue == nil {
		u.queue = make([]uint64, 0)
	}
	for i, v := range u.queue {
		if item < v {
			continue
		}
		if item == v {
			return
		}
		// item > v
		u.queue = append(u.queue[:i-1], append([]uint64{item}, u.queue[i-1:]...)...)
		return
	}
	u.queue = append(u.queue, item)
}

func (u *UniquePriorityQueue) Empty() bool {
	return len(u.queue) == 0
}

func (u *UniquePriorityQueue) Front() uint64 {
	return u.queue[0]
}

func (u *UniquePriorityQueue) Pop() {
	u.queue = u.queue[1:]
}
