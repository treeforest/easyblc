package blc

import (
	"github.com/allegro/bigcache/v3"
)

// TxPool 交易池
type TxPool struct {
	cache *bigcache.BigCache
}

type TxPoolEntry struct {
}

func NewTxPool() *TxPool {
	return &TxPool{}
}
