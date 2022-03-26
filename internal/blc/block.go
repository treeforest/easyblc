package blc

import (
	"bytes"
	"fmt"
	gob "github.com/treeforest/easyblc/pkg/gob"
	"github.com/treeforest/easyblc/pkg/merkle"
	log "github.com/treeforest/logger"
	"strconv"
	"time"
)

type Block struct {
	// header
	Version    int    // 版本号
	PreHash    []byte // 上一个区块哈希
	Hash       []byte // 区块哈希
	MerkelRoot []byte // 默克尔树根哈希
	Height     uint64 // 区块号,区块高度
	Time       int64  // 区块产生时的时间戳
	Bits       uint32 // 当前工作量证明的复杂度
	Nonce      uint64 // 挖矿找到的满足条件的值
	// body
	Transactions []*Transaction // 交易信息
	MerkleTree   *merkle.Tree   // 默克尔树
}

func CreateGenesisBlock(txs []*Transaction) (*Block, bool) {
	return NewBlock(0x1D00FFFF, 0, nil, txs)
}

func NewBlock(bits uint32, height uint64, preHash []byte, txs []*Transaction) (*Block, bool) {
	// TODO: 检查交易的输入输出是否合法
	block := &Block{
		Version:      1,
		PreHash:      preHash,
		Hash:         nil,
		Height:       height,
		Time:         time.Now().UnixNano(),
		Bits:         bits,
		Nonce:        0,
		Transactions: txs,
		MerkleTree:   nil,
	}
	block.BuildMerkleTree()
	succ := block.Mining()
	if !succ {
		return nil, false
	}
	return block, true
}

func (b *Block) Mining() bool {
	miner := NewProofOfWork(b)
	log.Debug("begin mining...")
	hash, nonce, succ := miner.Mining() // 挖矿
	if !succ {
		return false
	}
	b.Hash = hash
	b.Nonce = nonce
	log.Debug("nonce：", nonce)
	return true
}

func (b *Block) Marshal() ([]byte, error) {
	return gob.Encode(b)
}

func (b *Block) Unmarshal(data []byte) error {
	return gob.Decode(data, b)
}

func (b *Block) MarshalHeaderWithoutNonceAndHash() []byte {
	// 头结构 |version|preHash|height|time|merkelRoot|bits|
	data := bytes.Join([][]byte{
		[]byte(strconv.Itoa(b.Version)),
		b.PreHash,
		[]byte(strconv.FormatUint(b.Height, 10)),
		[]byte(strconv.FormatInt(b.Time, 10)),
		b.MerkelRoot,
		[]byte(fmt.Sprintf("%d", b.Bits)),
	}, []byte{})
	return data
}

func (b *Block) MarshalHeaderWithoutHash() []byte {
	data := b.MarshalHeaderWithoutNonceAndHash()
	return append(data, []byte(strconv.FormatUint(b.Nonce, 10))...)
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

func (b *Block) GetBlockTime() int64 {
	return b.Time
}
