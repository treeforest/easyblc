package blc

import "sync"

type UTXOSet struct {
	utxoSet map[[32]byte]map[int]TxOutput // TxHash => Index => TxOut
	Size    int                           // UTXO 个数
	locker  sync.RWMutex
}

func NewUTXOSet() *UTXOSet {
	return &UTXOSet{
		utxoSet: map[[32]byte]map[int]TxOutput{},
		Size:    0,
		locker:  sync.RWMutex{},
	}
}

func (s *UTXOSet) Traverse(fn func(txHash [32]byte, index int, output TxOutput)) {
	s.locker.RLock()
	defer s.locker.RUnlock()

	for txHash, outputs := range s.utxoSet {
		for index, output := range outputs {
			fn(txHash, index, output)
		}
	}
}

func (s *UTXOSet) Put(txHash [32]byte, index int, output TxOutput) {
	s.locker.Lock()
	defer s.locker.Unlock()

	if _, ok := s.utxoSet[txHash]; !ok {
		s.utxoSet[txHash] = make(map[int]TxOutput)
	}
	s.utxoSet[txHash][index] = output
	s.Size++
}

func (s *UTXOSet) Remove(txHash [32]byte, index int) {
	if !s.Exist(txHash, index) {
		return
	}

	s.locker.Lock()
	defer s.locker.Unlock()

	delete(s.utxoSet[txHash], index)
	if len(s.utxoSet[txHash]) == 0 {
		delete(s.utxoSet, txHash)
	}
	s.Size--
}

func (s *UTXOSet) Get(txHash [32]byte, index int) *TxOutput {
	if !s.Exist(txHash, index) {
		return nil
	}

	s.locker.RLock()
	out := s.utxoSet[txHash][index]
	s.locker.RUnlock()

	return &out
}

func (s *UTXOSet) Has(txHash [32]byte) bool {
	s.locker.RLock()
	_, ok := s.utxoSet[txHash]
	s.locker.RUnlock()

	return ok
}

func (s *UTXOSet) Exist(txHash [32]byte, index int) bool {
	s.locker.RLock()
	defer s.locker.RUnlock()

	if _, ok := s.utxoSet[txHash]; !ok {
		return false
	}
	if _, ok := s.utxoSet[txHash][index]; !ok {
		return false
	}
	return true
}

func (chain *BlockChain) resetUTXOSet() error {
	chain.utxoSet = NewUTXOSet()
	return chain.updateUTXOSet()
}

// updateUTXOSet 更新 utxo 集合
func (chain *BlockChain) updateUTXOSet() error {
	if chain.utxoSet == nil {
		chain.utxoSet = NewUTXOSet()
	}
	spent := make(map[[32]byte]map[uint32]struct{})

	// log.Debug("[blockchain utxo set]")
	err := chain.Traverse(func(block *Block) bool {
		for _, tx := range block.Transactions {
			// 交易输入
			for _, vin := range tx.Vins {
				if vin.IsCoinbase() {
					continue
				}
				if _, ok := spent[vin.TxId]; !ok {
					spent[vin.TxId] = make(map[uint32]struct{})
				}
				spent[vin.TxId][vin.Vout] = struct{}{}
			}
			// 交易输出
			for i, out := range tx.Vouts {
				if v, ok := spent[tx.Hash]; ok {
					if _, ok = v[uint32(i)]; ok {
						// 被花费
						delete(spent[tx.Hash], uint32(i))
						continue
					}
				}
				// log.Debugf("txid:%x\tindex:%d\taddress:%s\tvalue:%d", tx.Hash, i, out.Address, out.Value)
				chain.utxoSet.Put(tx.Hash, i, out)
			}
		}
		return true
	})
	if err != nil {
		return err
	}
	return nil
}

func (chain *BlockChain) GetBalance(address string) uint64 {
	utxoSet := chain.GetUTXOSet(address)

	var amount uint64 = 0
	utxoSet.Traverse(func(txHash [32]byte, index int, output TxOutput) {
		amount += output.Value
	})

	return amount
}

// GetUTXOSet 获取UTXO集合
func (chain *BlockChain) GetUTXOSet(address string) *UTXOSet {
	utxoSet := NewUTXOSet()
	chain.utxoSet.Traverse(func(txHash [32]byte, index int, output TxOutput) {
		if output.Address == address {
			utxoSet.Put(txHash, index, output)
		}
	})
	return utxoSet
}
