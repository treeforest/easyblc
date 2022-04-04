package blc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/treeforest/easyblc/pkg/merkle"
	log "github.com/treeforest/logger"
	"strconv"
	"time"
)

type Block struct {
	// header
	Version    int      // 版本号
	PreHash    [32]byte // 上一个区块哈希
	Hash       [32]byte // 区块哈希
	MerkelRoot []byte   // 默克尔树根哈希
	Height     uint64   // 区块号,区块高度
	Time       int64    // 区块产生时的时间戳
	Bits       uint32   // 当前工作量证明的复杂度
	Nonce      uint64   // 挖矿找到的满足条件的值
	// body
	Transactions []Transaction // 交易信息
	MerkleTree   *merkle.Tree  // 默克尔树
}

func CreateGenesisBlock(txs []Transaction) (*Block, bool) {
	return NewBlock(context.Background(), 0x1d00ffff, 0, [32]byte{}, txs)
	//return NewBlock(context.Background(), 0x1e00ffff, 0, [32]byte{}, txs)
}

func NewBlock(ctx context.Context, bits uint32, height uint64,
	preHash [32]byte, txs []Transaction) (*Block, bool) {

	block := &Block{
		Version:      1,
		PreHash:      preHash,
		Hash:         [32]byte{},
		Height:       height,
		Time:         time.Now().UnixNano(),
		Bits:         bits,
		Nonce:        0,
		Transactions: txs,
		MerkleTree:   nil,
	}

	block.BuildMerkleTree()
	succ := block.Mining(ctx)
	if !succ {
		return nil, false
	}
	return block, true
}

func (b *Block) Mining(ctx context.Context) bool {
	miner := NewProofOfWork(b)
	log.Debug("begin mining...")

	hash, nonce, succ := miner.Mining(ctx) // 挖矿

	if !succ {
		return false
	}

	b.Hash = hash
	b.Nonce = nonce
	log.Debug("nonce：", nonce)
	return true
}

func (b *Block) Marshal() ([]byte, error) {
	return json.Marshal(b)
}

func (b *Block) Unmarshal(data []byte) error {
	return json.Unmarshal(data, b)
}

func (b *Block) MarshalHeader() ([]byte, error) {
	block := *b
	block.Transactions = nil
	block.MerkleTree = nil
	return json.Marshal(&block)
}

func (b *Block) UnmarshalHeader(data []byte) error {
	return json.Unmarshal(data, b)
}

func (b *Block) MarshalHeaderWithoutNonceAndHash() []byte {
	// 头结构 |version|preHash|height|time|merkelRoot|bits|
	data := bytes.Join([][]byte{
		[]byte(strconv.Itoa(b.Version)),
		b.PreHash[:],
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

func (b *Block) CalculateHash() [32]byte {
	data := b.MarshalHeaderWithoutHash()
	return sha256.Sum256(data)
}

func (b *Block) BuildMerkleTree() {
	var txHashes [][]byte // 交易的哈希
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Hash[:])
	}
	tree := merkle.New()
	merkleRoot := tree.BuildWithHashes(txHashes)
	b.MerkleTree = tree
	b.MerkelRoot = merkleRoot
}

// CalculateMerkleRoot 计算交易哈希，当校验时使用
func (b *Block) CalculateMerkleRoot() []byte {
	var txHashes [][]byte // 交易的哈希
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Hash[:])
	}
	return merkle.New().BuildWithHashes(txHashes)
}

func (b *Block) GetBlockTime() int64 {
	return b.Time
}
