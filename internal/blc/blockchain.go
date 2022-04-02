package blc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/dao"
	"github.com/treeforest/easyblc/internal/blc/script"
	"github.com/treeforest/easyblc/pkg/utils"
	log "github.com/treeforest/logger"
	"sync"
	"time"
)

const (
	COIN = 100000000 // 1个币 = 100,000,000 聪
)

type BlockChain struct {
	dao         *dao.DAO // data access object
	utxoSet     *UTXOSet // utxo set
	txPool      *TxPool  // transaction pool
	latestBlock *Block   // 最新区块
	locker      sync.RWMutex
}

func GetBlockChain(dbPath string) *BlockChain {
	if dao.IsNotExistDB(dbPath) {
		return &BlockChain{
			dao:         dao.New(dbPath),
			utxoSet:     NewUTXOSet(),
			txPool:      NewTxPool(),
			latestBlock: nil,
		}
	}
	return MustGetExistBlockChain(dbPath)
}

func MustGetExistBlockChain(dbPath string) *BlockChain {
	if dao.IsNotExistDB(dbPath) {
		log.Fatal("block database not exist")
	}
	d, err := dao.Load(dbPath)
	if err != nil {
		log.Fatal("load block database failed: ", err)
	}
	blc := &BlockChain{
		dao:    d,
		txPool: NewTxPool(),
	}

	err = blc.updateUTXOSet()
	if err != nil {
		log.Fatal("find utxo set failed:", err)
	}

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

func CreateBlockChainWithGenesisBlock(dbPath, address string) *BlockChain {
	if !dao.IsNotExistDB(dbPath) {
		log.Fatal("block database already exist")
	}

	if !utils.IsValidAddress(address) {
		log.Fatalf("invalid address: %s", address)
	}

	d := dao.New(dbPath)
	blc := &BlockChain{
		dao:         d,
		utxoSet:     NewUTXOSet(),
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
	// 验证区块合法性
	if ok := chain.VerifyBlock(block); !ok {
		return fmt.Errorf("invalid block")
	}

	blockBytes, err := block.Marshal()
	if err != nil {
		return fmt.Errorf("block marshal failed:%v", err)
	}

	err = chain.dao.AddBlock(block.Height, block.Hash, blockBytes)
	if err != nil {
		return fmt.Errorf("add block to db failed:%v", err)
	}

	chain.locker.Lock()
	chain.latestBlock = block
	chain.locker.Unlock()

	// 更新交易池/UTXO
	for _, tx := range block.Transactions {
		chain.txPool.Remove(tx.Hash) // 从交易池中移除交易

		for _, vin := range tx.Vins {
			// 移除UTXO
			chain.utxoSet.Remove(vin.TxId, int(vin.Vout))
		}
	}

	return nil
}

func (chain *BlockChain) VerifyBlock(block *Block) bool {
	last := chain.GetLatestBlock()

	// 创世区块
	if last == nil {
		if block.Height != 0 {
			log.Warn("invalid block height")
			return false
		}
		if block.PreHash != [32]byte{} {
			log.Warn("invalid prehash with genesis block")
			return false
		}
		if block.Version != 1 {
			log.Warn("invalid version with genesis block")
			return false
		}
		// TODO: nBits 检查
		return true
	}

	// 检查区块高度
	if block.Height != last.Height+1 {
		log.Warnf("invalid block [height:%d] latestblock [height:%d]",
			block.Height, chain.GetLatestBlock().Height)
		return false
	}

	// 检查时间戳
	if block.Time < last.Time {
		log.Warn("invalid time, [%d] [%d]", block.Time, last.Time)
		return false
	}

	// 检查版本号
	if block.Version != last.Version {
		log.Warn("invalid version, [%d] [%d]", block.Version, last.Version)
		return false
	}

	// 检查难度目标值
	requiredBits := chain.GetNextWorkRequired()
	if block.Bits != requiredBits {
		log.Warn("invalid Bits, [%d] [%d]", block.Bits, last.Bits)
		return false
	}

	// 检查前哈希是否符合
	if block.PreHash != last.Hash {
		log.Warnf("invalid prehash, [%x] [%x]", block.PreHash, last.Hash)
		return false
	}

	// 至少得有一个coinbase交易
	if len(block.Transactions) < 1 {
		log.Warn("invalid transactions")
		return false
	}

	// 检查coinbase交易
	coinbaseTx := block.Transactions[0]
	if _, ok := coinbaseTx.IsCoinbase(); !ok {
		log.Warn("first tx is not coinbase")
		return false
	}

	// TODO: 不知道为什么传输破坏了数据，导致哈希总是计算不出来
	//requiredCoinbaseHash, err := coinbaseTx.CalculateHash()
	//if err != nil {
	//	log.Errorf("calculate coinbase tx hash: %v", err)
	//	return false
	//}
	//if coinbaseTx.Hash != requiredCoinbaseHash {
	//	log.Warnf("invalid coinbase txhash, [%x] [%x]", coinbaseTx.Hash, requiredCoinbaseHash)
	//	return false
	//}

	reward := chain.GetBlockSubsidy(block.Height)
	full, ok := block.Transactions[0].IsCoinbase()
	if !ok {
		log.Warn("first transaction is not coinbase tx")
		return false
	}

	fee := full - reward // 计算出使用的交易费，之后需要对比输入输出金额
	if fee < 0 {
		log.Warn("invalid fee")
		return false
	}

	// 检查交易输入
	var (
		inputAmount, outputAmount uint64 = 0, 0
	)
	for i := 1; i < len(block.Transactions); i++ {
		tx := block.Transactions[i]

		if _, ok = tx.IsCoinbase(); ok {
			// coinbase 交易必须放在第一个交易
			log.Warn("invalid coinbase transaction")
			return false
		}

		// 交易的哈希
		requiredTxHash, _ := tx.CalculateHash()
		if tx.Hash != requiredTxHash {
			log.Warnf("invalid txhash, [%x] [%x]", tx.Hash, requiredTxHash)
			return false
		}
		// TODO: 交易时间检查

		// 交易的输入输出数量
		if len(tx.Vins) == 0 || len(tx.Vouts) == 0 {
			log.Warn("invalid vins or vouts")
			return false
		}

		// 交易输入
		for _, vin := range tx.Vins {
			if vin.IsCoinbase() {
				log.Warn("invalid coinbase tx input")
				return false
			}

			txOut := chain.UTXOSet().Get(vin.TxId, int(vin.Vout))

			// 交易输入是否合法
			if txOut == nil {
				log.Warn("invalid tx input") // 没找到对应的交易输出
				return false
			}

			// 输入脚本是否合法
			ok := script.Verify(vin.TxId, vin.ScriptSig, txOut.ScriptPubKey)
			if !ok {
				log.Warn("invalid scriptSig")
				return false
			}

			// 记录输入金额
			inputAmount += txOut.Value
		}

		for _, vout := range tx.Vouts {
			if script.IsValidScriptPubKey(vout.ScriptPubKey) == false {
				log.Warn("invalid script pub key")
				return false
			}
			outputAmount += vout.Value
		}
	}

	// 输入输出金额对比
	if inputAmount-outputAmount != fee {
		log.Warn("invalid inputAmount outputAmount")
		return false
	}

	// 检查merkle tree
	if !bytes.Equal(block.CalculateMerkleRoot(), block.MerkelRoot) {
		log.Warn("invalid merkel root")
		return false
	}

	// 工作量证明，验证hash
	requiredBlockHash := block.CalculateHash()
	if block.Hash != requiredBlockHash {
		log.Warnf("invalid block hash, hash:%x required:%x", block.Hash, requiredBlockHash)
		return false
	}

	return true
}

func (chain *BlockChain) Close() {
	if err := chain.txPool.Close(); err != nil {
		log.Fatal("close tx pool failed: ", err)
	}
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
	chain.locker.RLock()
	defer chain.locker.RUnlock()

	if chain.latestBlock == nil {
		return nil
	}

	block := *chain.latestBlock
	return &block
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
	if tx.Hash != hash {
		log.Debug("transaction hash error")
		return 0, false
	}

	// 判断输入是否合法
	for _, vin := range tx.Vins {
		if vin.IsCoinbase() {
			continue
		}
		output := chain.utxoSet.Get(vin.TxId, int(vin.Vout))
		if output == nil {
			log.Debug("invalid transaction input")
			return 0, false
		}
		// 是否为有效地输入脚本
		ok := script.IsValidScriptSig(vin.ScriptSig)
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

func (chain *BlockChain) GetBlock(height uint64) (*Block, error) {
	if height < 0 || height > chain.GetLatestBlock().Height {
		return nil, errors.New("invalid height")
	}

	var block Block
	data, err := chain.dao.GetBlockByHeight(height)
	if err != nil {
		return nil, fmt.Errorf("get block by height failed: %v", err)
	}

	err = block.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal blokc data failed: %v", err)
	}

	return &block, nil
}

// GetBlockSubsidy 获取区块奖励
func (chain *BlockChain) GetBlockSubsidy(height uint64) uint64 {
	var halvings = height / 210000
	if halvings > 64 {
		return 0
	}
	subsidy := uint64(50 * COIN)
	subsidy = subsidy >> halvings // 每210000个区块奖励减半
	return subsidy
}

// MineBlock 挖矿
func (chain *BlockChain) MineBlock(ctx context.Context, address string) error {
	last := chain.GetLatestBlock()
	if last == nil {
		return errors.New("latest block is nil")
	}

	// 从交易池中取出交易, 并计算奖励：挖矿奖励+矿工费
	txs, minerFee := chain.ConsumeTxsFromTxPool(16)
	reward := chain.GetBlockSubsidy(chain.GetLatestBlock().Height + 1)

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

		block, succ = NewBlock(ctx, requiredBits, last.Height+1, last.Hash, append([]*Transaction{coinbaseTx}, txs...))

		// test
		//tmp, _ := json.MarshalIndent(block, "", "\t")
		//log.Debug(string(tmp))

		select {
		case <-ctx.Done():
			// 取消挖矿
			return nil
		default:
			select {
			case <-ctx.Done():
				return nil
			default:
			}
		}

		if !succ {
			// 挖矿失败，重新构建coinbase交易，继续挖矿
			continue
		}

		// 挖矿成功
		err = chain.AddBlock(block)
		if err != nil {
			return fmt.Errorf("add block to chain failed: %v", err)
		}
		return nil
	}
}

func (chain *BlockChain) GetTxPool() *TxPool {
	return chain.txPool
}

func (chain *BlockChain) UTXOSet() *UTXOSet {
	return chain.utxoSet
}
