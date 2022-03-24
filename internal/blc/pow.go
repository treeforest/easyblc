package blc

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strconv"
)

type ProofOfWork struct {
	block  *Block   // 目标区块
	target *big.Int // 难度值（哈希长度）
}

func NewProofOfWork(block *Block) *ProofOfWork {
	target := big.NewInt(1)
	// 左移256-Bits, 计算出的哈希满足小于target即挖矿成功
	target = target.Lsh(target, 256-block.Bits)
	return &ProofOfWork{block: block, target: target}
}

// Mining 挖矿
func (pow *ProofOfWork) Mining() ([]byte, uint64) {
	var (
		data    []byte
		nonce   uint64   = 0 // 碰撞次数
		hash    [32]byte     // 计算出的区块哈希值
		hashInt big.Int      // 哈希对应的大整数
	)
	// 组装挖矿结构 |preHash|height|timestamp|merkelRoot|targetBit|nonce|
	data = bytes.Join([][]byte{
		pow.block.PreHash,
		[]byte(strconv.FormatInt(pow.block.Height, 10)),
		[]byte(strconv.FormatInt(pow.block.Time, 10)),
		pow.block.MerkelRoot,
		[]byte(fmt.Sprintf("%d", pow.block.Bits)),
	}, []byte{})
	for {
		hash = sha256.Sum256(append(data, []byte(strconv.FormatUint(nonce, 10))...))
		hashInt.SetBytes(hash[:])
		if pow.target.Cmp(&hashInt) == 1 {
			break // 挖到区块
		}
		nonce++
	}
	return hash[:], nonce
}
