package blc

import (
	"crypto/sha256"
	"math"
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
func (pow *ProofOfWork) Mining() ([]byte, uint64, bool) {
	var (
		data    []byte
		nonce   uint64   = 0 // 碰撞次数
		hash    [32]byte     // 计算出的区块哈希值
		hashInt big.Int      // 哈希对应的大整数
	)
	// 组装挖矿结构 |preHash|height|timestamp|merkelRoot|targetBit|nonce|
	data = pow.block.MarshalHeaderWithoutNonceAndHash()
	for {
		hash = sha256.Sum256(append(data, []byte(strconv.FormatUint(nonce, 10))...))
		hashInt.SetBytes(hash[:])
		if pow.target.Cmp(&hashInt) == 1 {
			break // 挖到区块
		}
		nonce++
		if nonce == math.MaxUint64 {
			// 当前组合的最大值都没能挖出区块， 需要进一步修改coinbase data
			return nil, 0, false
		}
	}
	return hash[:], nonce, true
}

// GetNextWorkRequired 计算难度目标值，规定30秒出一个块
func (pow *ProofOfWork) GetNextWorkRequired() {
	// TODO
}

// CalculateNextWorkRequired 计算下个区块的目标值
func (pow *ProofOfWork) CalculateNextWorkRequired() {
	// TODO
}