package blc

import (
	"context"
	"crypto/sha256"
	log "github.com/treeforest/logger"
	"math"
	"math/big"
	"runtime"
	"strconv"
	"sync"
	"time"
)

type ProofOfWork struct {
	block  *Block   // 目标区块
	target *big.Int // 难度值（哈希长度）
}

func NewProofOfWork(block *Block) *ProofOfWork {
	//target := big.NewInt(1)
	//// 左移256-Bits, 计算出的哈希满足小于target即挖矿成功
	//target = target.Lsh(target, 256-block.Bits)
	target := UnCompact(block.Bits)
	return &ProofOfWork{block: block, target: target}
}

// Mining 挖矿
func (pow *ProofOfWork) Mining() ([32]byte, uint64, bool) {
	// 组装挖矿结构 |preHash|height|timestamp|merkelRoot|targetBit|nonce|
	data := pow.block.MarshalHeaderWithoutNonceAndHash()
	wg := sync.WaitGroup{}
	ctx, cancel := context.WithCancel(context.Background())
	nonceCh := make(chan uint64, 1)
	done := make(chan struct{}, 2)
	now := time.Now()

	// 开n个协程同时挖矿
	n := runtime.NumCPU()
	wg.Add(n)
	for i := uint64(0); i < uint64(n); i++ {
		begin := math.MaxUint64 / uint64(n) * i
		end := math.MaxUint64 / uint64(n) * (i + 1)
		go pow.mining(ctx, &wg, begin, end, data, nonceCh)
	}
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()

	for {
		select {
		case nonce := <-nonceCh:
			sub := time.Now().Sub(now)
			log.Debugf("挖矿用时: %fs(%dms)", sub.Seconds(), sub.Milliseconds())

			cancel() // 退出协程
			hash := sha256.Sum256(append(data, []byte(strconv.FormatUint(nonce, 10))...))
			return hash, nonce, true
		case <-done:
			// 需要进一步修改coinbase data，然后再次挖矿
			cancel()
			return [32]byte{}, 0, false
		}
	}
}

func (pow *ProofOfWork) mining(ctx context.Context, wg *sync.WaitGroup, begin, end uint64, data []byte, nonceChan chan uint64) {
	defer wg.Done()

	var hash [32]byte   // 计算出的区块哈希值
	var hashInt big.Int // 哈希对应的大整数
	nonce := begin
	for {
		select {
		case <-ctx.Done():
			return
		default:
			hash = sha256.Sum256(append(data, []byte(strconv.FormatUint(nonce, 10))...))
			hashInt.SetBytes(hash[:])
			if pow.target.Cmp(&hashInt) == 1 {
				nonceChan <- nonce // 挖到区块
				return
			}
			nonce++
			if nonce == end {
				// 当前组合的最大值都没能挖出区块
				return
			}
		}
	}
}

const (
	// DifficultyAdjustmentInterval 难度值调整区块间隔
	DifficultyAdjustmentInterval = uint64(2016)
	// PowTargetTimespan 预定出2100个块规定的时间（每个出块时间大约为10秒钟）
	PowTargetTimespan = 10 * 2016
)

var (
	// PowLimit 最低难度值
	PowLimit = UnCompact(uint32(0x1e00ffff))
)

// GetNextWorkRequired 计算难度目标值，规定10分钟出一个块
func (chain *BlockChain) GetNextWorkRequired() uint32 {
	lastBlock := chain.GetLatestBlock()
	if (lastBlock.Height+1)%DifficultyAdjustmentInterval != 0 {
		// 不需要难度调整，使用上一个区块的难度值
		return lastBlock.Bits
	}
	// 找到最近2016个区块的首个区块
	heightFirst := lastBlock.Height - (DifficultyAdjustmentInterval - 1)
	firstBlock, _ := chain.GetAncestor(heightFirst)
	// 重新计算难度值
	return chain.CalculateNextWorkRequired(lastBlock, firstBlock)
}

// CalculateNextWorkRequired 计算难度值
func (chain *BlockChain) CalculateNextWorkRequired(latest *Block, first *Block) uint32 {
	// 如果最近2016个区块生成时间小于1/4，就按照1/4来计算，大于4倍，就按照4倍来计算，以防止出现难度偏差过大的现象
	actualTimespan := latest.GetBlockTime() - first.GetBlockTime()
	if actualTimespan < PowTargetTimespan/4 {
		actualTimespan = PowTargetTimespan / 4
	}
	if actualTimespan > PowTargetTimespan*4 {
		actualTimespan = PowTargetTimespan * 4
	}

	// 调整难度值
	newTarget := UnCompact(latest.Bits)
	newTarget.Mul(newTarget, big.NewInt(actualTimespan))
	newTarget.Div(newTarget, big.NewInt(PowTargetTimespan))

	// target 越大，挖矿难度越低。这里要求最低难度不能大于 PowLimit
	if newTarget.Cmp(PowLimit) == 1 {
		newTarget = PowLimit
	}

	return Compact(newTarget)
}

// UnCompact 对目标值进行解压缩，返回对应的难度值
func UnCompact(bits uint32) *big.Int {
	high := bits & 0xFF000000 >> 24 // 指数
	low := bits & 0x00FFFFFF        // 系数
	x := big.NewInt(int64(low))
	y := big.NewInt(0).Lsh(big.NewInt(1), uint(8*(high-3)))
	target := big.NewInt(0).Mul(x, y)
	return target
}

// Compact 对难度值进行压缩，返回对应的目标值
func Compact(target *big.Int) uint32 {
	bs := target.Bytes()
	exponent := uint(0) // 指数
	for i := len(bs) - 1; i > 0; i-- {
		if bs[i] == 0x00 {
			exponent += 8
			continue
		}
		v := bs[i]
		for {
			// 最低位是否为0，即 0^1=0
			if v^0x1 == 0 {
				exponent++
				v = v >> 1
				continue
			}
			break
		}
		break
	}

	y := big.NewInt(0).Lsh(big.NewInt(1), exponent)
	x := target.Div(target, y)
	high := uint32((exponent/8 + 3) << 24 & 0xFF000000) // 高位
	low := uint32(x.Int64() & 0x00FFFFFF)
	bits := high | low
	return bits
}
