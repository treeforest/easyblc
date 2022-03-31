package blc

import (
	"errors"
	"sync"
)

// TxPool 交易池
type TxPool struct {
	mu    sync.Mutex                // locker
	Fees  *UniquePriorityQueue      // sorted Fees,priority Queue
	Cache map[uint64][]*TxPoolEntry // fee => txs
	Cap   int                       // 最大容量
	Size  int                       // 当前容量
}

type TxPoolEntry struct {
	Fee uint64
	Tx  *Transaction
}

func NewTxPool() *TxPool {
	pool := &TxPool{
		Fees:  NewUniquePriorityQueue(),
		Cache: map[uint64][]*TxPoolEntry{},
		Cap:   4096,
		Size:  0,
	}
	return pool
}

// AddToTxPool 将交易放入内存池
func (chain *BlockChain) AddToTxPool(tx *Transaction) error {
	// 输出是否已经在交易池中被使用
	for _, vin := range tx.Vins {
		if chain.txPool.IsUsedTxInput(vin) {
			return errors.New("tx input is used")
		}
	}
	// 输入是否是有效的
	fee, ok := chain.IsValidTx(tx)
	if !ok {
		return errors.New("invalid transaction")
	}
	chain.txPool.Put(fee, &TxPoolEntry{Fee: fee, Tx: tx})
	return nil
}

// ConsumeTxsFromTxPool 从交易池中获取交易
func (chain *BlockChain) ConsumeTxsFromTxPool(count int) ([]*Transaction, uint64) {
	if count <= 0 {
		return nil, 0
	}
	if count > 256 {
		count = 256 // 一次最多只能获取256个交易
	}

	minerFee := uint64(0)
	txs := make([]*Transaction, 0)

	entries := chain.txPool.Get(count)
	for _, entry := range entries {
		legal := true

		for _, vin := range entry.Tx.Vins {
			if !chain.utxoSet.Exist(vin.TxId, int(vin.Vout)) {
				// utxo 集合中没有对应的输出，为非法交易
				legal = false
				break
			}
		}

		if !legal {
			continue
		}

		minerFee += entry.Fee
		txs = append(txs, entry.Tx)
	}

	return txs, minerFee
}

func (pool *TxPool) Traverse(fn func(fee uint64, tx *Transaction)) {
	fees := *pool.Fees
	for !fees.Empty() {
		fee := fees.Front()
		fees.Pop()
		for _, entry := range pool.Cache[fee] {
			fn(fee, entry.Tx)
		}
	}
}

// IsUsedTxInput 是否为使用过的交易输入
func (pool *TxPool) IsUsedTxInput(in *TxInput) bool {
	used := false
	pool.Traverse(func(fee uint64, tx *Transaction) {
		for _, v := range tx.Vins {
			if in.TxId == v.TxId {
				used = true
				return
			}
		}
	})
	return used
}

// Put 将交易存入交易池
func (pool *TxPool) Put(fee uint64, entry *TxPoolEntry) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.Fees.Push(fee)

	if pool.Cache == nil {
		pool.Cache = make(map[uint64][]*TxPoolEntry)
	}

	if _, ok := pool.Cache[fee]; !ok {
		pool.Cache[fee] = []*TxPoolEntry{entry}
		return
	}

	for _, v := range pool.Cache[fee] {
		if v.Tx.Hash == entry.Tx.Hash {
			// transaction already exist in txPool
			return
		}
	}

	pool.Cache[fee] = append(pool.Cache[fee], entry)
}

// Get 从交易池中获取交易
// 参数：
//		count: 获取多少个交易，将返回小于等于交易的个数。
func (pool *TxPool) Get(count int) []*TxPoolEntry {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	var (
		counter = 0
		i       int
		key     uint64
	)
	txs := make([]*TxPoolEntry, 0, count)

	for !pool.Empty() {
		key = pool.Fees.Front()

		for i = 0; i < len(pool.Cache[key]); i++ {
			txs = append(txs, pool.Cache[key][i])
			counter++
			if counter == count {
				pool.Cache[key] = pool.Cache[key][i+1:] // remove used
				if len(pool.Cache) == 0 {
					delete(pool.Cache, key)
				}
				return txs
			}
		}

		delete(pool.Cache, key)
		pool.Fees.Pop() // dequeue
	}

	return txs
}

func (pool *TxPool) Remove(txid [32]byte) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	for fee, entries := range pool.Cache {
		for i, entry := range entries {
			if entry.Tx.Hash == txid {
				if len(entries) == 1 {
					delete(pool.Cache, fee)
					pool.Fees.Remove(fee)
				} else {
					pool.Cache[fee] = append(entries[:i], entries[i+1:]...)
				}
				return
			}
		}
	}
}

func (pool *TxPool) Empty() bool {
	return len(pool.Cache) == 0 && pool.Fees.Empty()
}

func (pool *TxPool) Close() error {
	return nil
}

// UniquePriorityQueue 唯一元素的优先级队列, 按照从大到小排列
type UniquePriorityQueue struct {
	Queue []uint64
}

func NewUniquePriorityQueue() *UniquePriorityQueue {
	return &UniquePriorityQueue{Queue: make([]uint64, 0)}
}

func (u *UniquePriorityQueue) Push(item uint64) {
	if u.Queue == nil {
		u.Queue = make([]uint64, 0)
	}
	for i, v := range u.Queue {
		if item < v {
			continue
		}
		if item == v {
			return
		}
		// item > v
		u.Queue = append(u.Queue[:i-1], append([]uint64{item}, u.Queue[i-1:]...)...)
		return
	}
	u.Queue = append(u.Queue, item)
}

func (u *UniquePriorityQueue) Remove(item uint64) {
	for i, v := range u.Queue {
		if v == item {
			u.Queue = append(u.Queue[:i], u.Queue[i+1:]...)
			return
		}
	}
}

func (u *UniquePriorityQueue) Empty() bool {
	return len(u.Queue) == 0
}

func (u *UniquePriorityQueue) Front() uint64 {
	return u.Queue[0]
}

func (u *UniquePriorityQueue) Pop() {
	u.Queue = u.Queue[1:]
}
