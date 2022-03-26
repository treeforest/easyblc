package blc

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/dao"
	"github.com/treeforest/easyblc/internal/blc/script"
	"github.com/treeforest/easyblc/pkg/utils"
	log "github.com/treeforest/logger"
	"time"
)

const (
	COIN = 100000000 // 1个币 = 100,000,000 聪
)

type BlockChain struct {
	dao         *dao.DAO // data access object
	utxoSet     UTXOSet  // utxo set
	txPool      *TxPool  // transaction pool
	latestBlock *Block   // 最新区块
}

func GetBlockChain() *BlockChain {
	if dao.IsNotExistDB() {
		log.Fatal("block database not exist")
	}
	d, err := dao.Load()
	if err != nil {
		log.Fatal("load block database failed: ", err)
	}
	blc := &BlockChain{dao: d, txPool: NewTxPool()}

	utxoSet, err := blc.FindAllUTXOSet()
	if err != nil {
		log.Fatal("find utxo set failed:", err)
	}
	blc.utxoSet = utxoSet

	data, err := blc.dao.GetBlock(blc.dao.GetLatestBlockHash())
	if err != nil {
		log.Fatal("get block failed:", err)
	}
	var block Block
	err = block.Unmarshal(data)
	if err != nil {
		log.Fatal("block unmarshal failed:", err)
	}
	blc.latestBlock = &block

	return blc
}

func CreateBlockChainWithGenesisBlock(address string) *BlockChain {
	if !dao.IsNotExistDB() {
		log.Fatal("block database already exist")
	}

	if !utils.IsValidAddress(address) {
		log.Fatalf("invalid address: %s", address)
	}

	blc := &BlockChain{
		dao:         dao.New(),
		utxoSet:     UTXOSet{},
		txPool:      NewTxPool(),
		latestBlock: nil,
	}

	reward := blc.GetBlockSubsidy(0)
	coinbaseTx, err := NewCoinbaseTransaction(reward, address, []byte("挖矿不容易，且挖且珍惜"))
	if err != nil {
		log.Fatal("create coinbase transaction failed:", err)
	}

	block, succ := CreateGenesisBlock([]*Transaction{coinbaseTx})
	if !succ {
		log.Fatal("create Genesis Block failed")
	}

	err = blc.AddBlock(block)
	if err != nil {
		log.Fatal("add block to chain failed:", err)
	}

	return blc
}

func (chain *BlockChain) AddBlock(block *Block) error {
	blockBytes, err := block.Marshal()
	if err != nil {
		return fmt.Errorf("block marshal failed:%v", err)
	}
	err = chain.dao.AddBlock(block.Hash, blockBytes)
	if err != nil {
		return fmt.Errorf("add block to db failed:%v", err)
	}
	return nil
}

func (chain *BlockChain) Close() {
	if err := chain.dao.Close(); err != nil {
		log.Fatal("close database failed: ", err)
	}
}

// GetAncestor 获取指定高度的祖先区块
func (chain *BlockChain) GetAncestor(height uint64) (*Block, error) {
	if height < 0 || chain.GetLatestBlock().Height < height {
		return nil, errors.New("invalid height")
	}
	var block *Block
	err := chain.Traverse(func(b *Block) {
		if b.Height == height {
			block = b
			return
		}
	})
	if err != nil {
		return nil, fmt.Errorf("traverse blockchain error:%v", err)
	}
	return block, nil
}

// GetLatestBlock 获取最新区块
func (chain *BlockChain) GetLatestBlock() *Block {
	return chain.latestBlock
}

// GetBlockIterator 返回区块迭代器
func (chain *BlockChain) GetBlockIterator() *BlockIterator {
	return NewBlockIterator(chain.dao)
}

// Traverse 遍历区块链
func (chain *BlockChain) Traverse(fn func(block *Block)) error {
	it := chain.GetBlockIterator()
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
func (chain *BlockChain) IsValidTx(tx *Transaction) (uint64, bool) {
	var inputAmount, outputAmount, fee uint64

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
		outputs, ok := chain.utxoSet[string(vin.TxId)]
		if !ok {
			log.Debugf("not found utxo => txid[%x]", vin.TxId)
			return 0, false
		}
		output, ok := outputs[int(vin.Vout)]
		if !ok {
			// 交易id没有对应的utxo
			log.Debug("not found txoutput")
			return 0, false
		}
		// 是否为有效地输入脚本
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
		// 是否为有效地输出脚本
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

func (chain *BlockChain) GetBlockSubsidy(height uint64) uint64 {
	var halvings uint64 = height / 210000
	if halvings > 64 {
		return 0
	}
	subsidy := uint64(50 * COIN)
	subsidy = subsidy >> halvings // 每210000个区块奖励减半
	return subsidy
}

// MineBlock 挖矿
func (chain *BlockChain) MineBlock(address string) error {
	if chain.latestBlock == nil {
		return errors.New("latest block is nil")
	}

	var reward, minerFee uint64 = 0, 0
	txs := make([]*Transaction, 0)

	// 从交易池中取出交易,并计算奖励：挖矿奖励+矿工费
	entries := chain.GetTxsFromTxPool(16)
	for _, entry := range entries {
		minerFee += entry.Fee
		txs = append(txs, entry.Tx)
	}
	reward = chain.GetBlockSubsidy(chain.latestBlock.Height + 1)

	var (
		block *Block
		succ  bool // 挖矿是否成功
	)
	for {
		// 构造创币交易
		coinbaseTx, err := NewCoinbaseTransaction(reward+minerFee, address, []byte(time.Now().String()))
		if err != nil {
			log.Fatal("create coinbase transaction failed:", err)
		}
		requiredBits := chain.GetNextWorkRequired()
		block, succ = NewBlock(requiredBits, chain.latestBlock.Height+1, chain.latestBlock.Hash,
			append([]*Transaction{coinbaseTx}, txs...))
		if succ {
			break
		}
	}

	err := chain.AddBlock(block)
	if err != nil {
		return fmt.Errorf("add block to chain failed: %v", err)
	}

	return nil
}
