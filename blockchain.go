package blc

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/treeforest/easyblc/dao"
	"github.com/treeforest/easyblc/script"
	log "github.com/treeforest/logger"
	"math/big"
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

	err = blc.loadLatestBlock()
	if err != nil {
		log.Fatal("local latest block failed:", err)
	}

	return blc
}

func CreateBlockChainWithGenesisBlock(dbPath, address string) *BlockChain {
	if dao.IsNotExistDB(dbPath) {
		log.Fatal("block database not exist")
	}

	d := dao.New(dbPath)
	blc := &BlockChain{
		dao:         d,
		utxoSet:     NewUTXOSet(),
		txPool:      NewTxPool(),
		latestBlock: nil,
	}

	blc.MineGenesisBlock(context.Background(), address)
	return blc
}

func (chain *BlockChain) MineGenesisBlock(ctx context.Context, address string) {
	if !IsValidAddress(address) {
		log.Fatalf("invalid address: %s", address)
	}

	reward := chain.GetBlockSubsidy(0)
	coinbaseTx, err := NewCoinbaseTransaction(reward, address, []byte("挖矿不容易，且挖且珍惜"))
	if err != nil {
		log.Fatal("create coinbase transaction failed:", err)
	}

	block, succ := CreateGenesisBlock(ctx, []Transaction{*coinbaseTx})
	if !succ {
		log.Fatal("create Genesis Block failed")
	}

	err = chain.AddBlock(block)
	if err != nil {
		log.Fatal("add block to chain failed:", err)
	}
}

func (chain *BlockChain) loadLatestBlock() error {
	data, err := chain.dao.GetBlock(chain.dao.GetLatestBlockHash())
	if err != nil {
		return fmt.Errorf("get latest block failed: %v", err)
	}
	var block Block
	err = block.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("block unmarshal failed: %v", err)
	}
	chain.locker.Lock()
	chain.latestBlock = &block
	chain.locker.Unlock()
	return nil
}

func (chain *BlockChain) AddBlock(block *Block) error {
	preBlock, err := chain.GetAncestor(block.Height - 1)
	if err != nil {
		return errors.WithStack(err)
	}

	// 验证区块合法性
	if ok := chain.VerifyBlock(preBlock, block); !ok {
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

func (chain *BlockChain) VerifyBlock(preBlock, block *Block) bool {
	if preBlock == nil || block == nil {
		return false
	}

	// 若是初始区块
	if block.Height == 0 {
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

	// 1、检查hash
	hashBytes := block.CalculateHash()
	if hashBytes != block.Hash {
		log.Warn("block hash error, required:%s actual:%s", hashBytes, block.Hash)
		return false
	}

	// 2、检查难度目标值
	nBits, err := chain.GetWorkRequired(block.Height)
	if err != nil {
		log.Warn("get work required failed:", err)
		return false
	}
	if nBits != block.Bits {
		log.Warn("bits not equal， required:%d actual:%d", nBits, block.Bits)
		return false
	}

	// 3、检查工作量证明
	target := UnCompact(block.Bits)
	hashInt := new(big.Int).SetBytes(hashBytes[:])
	if target.Cmp(hashInt) != 1 {
		log.Warn("nonce error")
		return false
	}

	// 4、检查前preHash
	//preBlock, err := chain.GetAncestor(block.Height - 1)
	//if err != nil {
	//	return false
	//}
	if block.PreHash != preBlock.Hash {
		return false
	}

	// 5、检查时间戳，需要小于父区块的时间戳
	if block.Time < preBlock.Time {
		log.Warn("invalid time, [%d] [%d]", block.Time, preBlock.Time)
		return false
	}

	// 6、检查coinbase交易
	// 6.1 至少得有一个coinbase交易
	if len(block.Transactions) < 1 {
		log.Warn("invalid transactions")
		return false
	}
	// 6.2 第一个交易应该是coinbase交易
	coinbaseTx := block.Transactions[0]
	if _, ok := coinbaseTx.IsCoinbase(); !ok {
		log.Warn("first tx is not coinbase")
		return false
	}
	// 6.3 检查coinbase哈希值
	requiredCoinbaseHash, err := coinbaseTx.CalculateHash()
	if err != nil {
		log.Errorf("calculate coinbase tx hash: %v", err)
		return false
	}
	if coinbaseTx.Hash != requiredCoinbaseHash {
		log.Warnf("invalid coinbase txhash, [%x] [%x]", coinbaseTx.Hash, requiredCoinbaseHash)
		return false
	}

	// 7. 检查矿工费（交易费+区块奖励）
	// 7.1 获得区块奖励
	reward := chain.GetBlockSubsidy(block.Height)
	full, ok := block.Transactions[0].IsCoinbase()
	if !ok {
		log.Warn("first transaction is not coinbase tx")
		return false
	}
	// 7.2 获得交易费
	fee := full - reward
	if fee < 0 {
		log.Warn("invalid fee")
		return false
	}
	// 7.3 解析出输入输出金额
	var inputAmount, outputAmount uint64 = 0, 0
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
			ok = script.Verify(vin.TxId, vin.ScriptSig, txOut.ScriptPubKey)
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
	// 7.4 矿工费用 = 交易输入金额 - 交易输出金额
	if inputAmount-outputAmount != fee {
		log.Warn("invalid inputAmount outputAmount")
		return false
	}

	// 8、检查默克尔树 merkle tree
	if !bytes.Equal(block.CalculateMerkleRoot(), block.MerkelRoot) {
		log.Warn("invalid merkel root")
		return false
	}

	return true
}

var once sync.Once

func (chain *BlockChain) Close() {
	once.Do(func() {
		if err := chain.txPool.Close(); err != nil {
			log.Fatal("close tx pool failed: ", err)
		}
		if err := chain.dao.Close(); err != nil {
			log.Fatal("close database failed: ", err)
		}
	})
}

// GetAncestor 获取指定高度区块
func (chain *BlockChain) GetAncestor(height uint64) (*Block, error) {
	data, err := chain.dao.GetBlockByHeight(height)
	if err != nil {
		return nil, errors.New("invalid height")
	}
	var block Block
	err = block.Unmarshal(data)
	if err != nil {
		return nil, fmt.Errorf("unmarshal block failed:%v", err)
	}
	return &block, nil
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

// Traverse 遍历区块链,若返回false，则终止遍历
func (chain *BlockChain) Traverse(fn func(block *Block) bool) error {
	it := chain.GetBlockIterator()
	for {
		b, err := it.Next()
		if err != nil {
			return fmt.Errorf("get next block error[%v]", err)
		}
		if b == nil {
			break
		}
		if fn(b) == false {
			return nil
		}
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
		log.Debugf("transaction hash error, dst[%x] src[%x]", hash, tx.Hash)
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
		if !IsValidAddress(vout.Address) {
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

// RemoveBlockFrom 移除区块高度及往后的区块，当产生分叉，需要合并链时调用
func (chain *BlockChain) RemoveBlockFrom(start uint64) error {
	height := chain.GetLatestBlock().Height
	if start < 0 || start > height {
		return nil
	}

	for ; height >= start; height-- {
		// TODO: 回收交易 或者 通过网络同步
		// 移除区块
		_ = chain.dao.RemoveLatestBlock()
	}

	// 重新设置 latest block
	err := chain.loadLatestBlock()
	if err != nil {
		return fmt.Errorf("local latest block failed:%v", err)
	}

	// 重置utxo结合
	err = chain.resetUTXOSet()
	if err != nil {
		return fmt.Errorf("reset utxo set failed:%v", err)
	}

	return nil
}

func (chain *BlockChain) GetBlock(height uint64) (*Block, error) {
	if chain.GetLatestBlock() == nil {
		return nil, errors.New("blockchain is null")
	}

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

		block, succ = NewBlock(ctx, requiredBits, last.Height+1, last.Hash, append([]Transaction{*coinbaseTx}, txs...))

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
