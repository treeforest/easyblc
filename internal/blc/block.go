package blc

import (
	gob "github.com/treeforest/easyblc/pkg/gob"
	"github.com/treeforest/easyblc/pkg/merkle"
	log "github.com/treeforest/logger"
	"time"
)

type Block struct {
	// header
	Version    int    // 版本号
	PreHash    []byte // 上一个区块哈希
	Hash       []byte // 区块哈希
	MerkelRoot []byte // 默克尔树根哈希
	Height     int64  // 区块号
	Time       int64  // 区块产生时的时间戳
	Bits       uint   // 当前工作量证明的复杂度
	Nonce      uint64 // 挖矿找到的满足条件的值
	// body
	Transactions []*Transaction // 交易信息
	MerkleTree   *merkle.Tree   // 默克尔树
}

func CreateGenesisBlock(txs []*Transaction) *Block {
	return NewBlock(0, nil, txs)
}

func NewBlock(height int64, preHash []byte, txs []*Transaction) *Block {
	// TODO: 检查交易的输入输出是否合法
	block := &Block{
		Version:      1,
		PreHash:      preHash,
		Hash:         nil,
		Height:       height,
		Time:         time.Now().UnixNano(),
		Bits:         16,
		Nonce:        0,
		Transactions: txs,
		MerkleTree:   nil,
	}
	block.BuildMerkleTree()
	block.Mining()
	return block
}

func (b *Block) Mining() {
	miner := NewProofOfWork(b)
	hash, nonce := miner.Mining() // 挖矿
	b.Hash = hash
	b.Nonce = nonce
	log.Debug("碰撞次数：", nonce)
}

func (b *Block) Marshal() ([]byte, error) {
	return gob.Encode(b)
}

func (b *Block) Unmarshal(data []byte) {
	gob.Decode(data, b)
}

func (b *Block) BuildMerkleTree() {
	var txHashes [][]byte // 交易的哈希
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Hash)
	}
	tree := merkle.New()
	merkleRoot := tree.BuildWithHashes(txHashes)
	b.MerkleTree = tree
	b.MerkelRoot = merkleRoot
}
