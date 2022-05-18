package blc

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/treeforest/easyblc/internal/blc/config"
	"github.com/treeforest/easyblc/internal/blc/proto/p2p"
	"github.com/treeforest/easyblc/pkg/graceful"
	"github.com/treeforest/gossip"
	log "github.com/treeforest/logger"
	"sync"
	"time"
)

const (
	// defPerGossipNum 每次广播的gossip消息数量
	defPerGossipNum = 20
	// defPerSummaryBlockHashCount 每次summary发送的区块哈希的数量
	defPerSummaryBlockHashNum = 100
	// defPerPullBlockNum 每次拉取的区块数量为20
	defPerPullBlockNum = 50
)

// P2PServer 点对点服务
type P2PServer struct {
	sync.RWMutex
	gossipSrv       gossip.Gossip
	Id              string
	broadcasts      [][]byte          // 广播队列
	chain           *BlockChain       // 区块链对象
	metadata        map[string]string // 节点元数据
	nodeType        p2p.NodeType      // 节点类型
	rewardAddress   string            // 挖矿奖励地址
	startMiningChan chan struct{}     // 开始挖矿
	stopMiningChan  chan struct{}     // 停止挖矿
	accept          chan *p2p.Message // 消息通道
	conf            *config.Config
	stop            chan struct{}
	stopOnce        sync.Once
}

func NewP2PServer(conf *config.Config, chain *BlockChain) *P2PServer {
	id := uuid.NewString()
	gossipConf := gossip.DefaultConfig()
	gossipConf.Id = id
	gossipConf.Port = conf.Port
	gossipConf.Endpoint = conf.Endpoint
	gossipConf.BootstrapPeers = conf.BootstrapPeers

	server := &P2PServer{
		Id:              id,
		gossipSrv:       gossip.New(gossipConf),
		broadcasts:      make([][]byte, 1024),
		chain:           chain,
		metadata:        map[string]string{},
		nodeType:        p2p.NodeType(conf.Type),
		rewardAddress:   conf.RewardAddress,
		startMiningChan: make(chan struct{}, 1),
		stopMiningChan:  make(chan struct{}, 1),
		accept:          make(chan *p2p.Message, 256),
		conf:            conf,
		stop:            make(chan struct{}, 1),
	}

	server.chain.GetTxPool().SetPutCallback(server.gossipNewTransaction)

	return server
}

func (s *P2PServer) Run() {
	go s.dispatch()
	time.Sleep(time.Millisecond * 100)

	if s.nodeType == p2p.NodeType_Miner {
		go s.mining()
		s.startMining()
	}

	graceful.Stop(func() {
		log.Info("graceful stopping...")
		s.Stop()
	})
}

func (s *P2PServer) Stop() {
	s.stopOnce.Do(func() {
		if s.nodeType == p2p.NodeType_Miner {
			s.stopMiningChan <- struct{}{}
		}
		close(s.stop)
		s.gossipSrv.Stop()
		s.chain.Close()
	})
}

// NotifyMsg 处理用户数据
// 参数：
// 		msg: 用户数据
func (s *P2PServer) NotifyMsg(data []byte) {
	if data == nil || len(data) == 0 {
		return
	}
	msg := &p2p.Message{}
	if err := msg.Unmarshal(data); err != nil {
		log.Errorf("NotifyMsg unmarshal p2pMessage failed: %v", err)
		return
	}
	s.accept <- msg
}

// GetBroadcasts 返回需要进行gossip广播的数据。
// 返回值：
// 	   data: 待广播的消息
func (s *P2PServer) GetBroadcasts() (data [][]byte) {
	var broadcasts [][]byte
	s.Lock()
	if len(s.broadcasts) > defPerGossipNum {
		broadcasts = s.broadcasts[:defPerGossipNum]
		s.broadcasts = s.broadcasts[defPerGossipNum:]
	} else {
		broadcasts = s.broadcasts
		s.broadcasts = [][]byte{}
	}
	s.Unlock()
	return broadcasts
}

// Summary 返回 pull 请求时所携带的信息。
// 返回值：
//     data: pull请求信息
func (s *P2PServer) Summary() (data []byte) {
	// 获取当前交易池中所有交易的hash
	hashes := s.chain.GetTxPool().TxHashes()
	mp := make(map[string]p2p.Empty, len(hashes))
	for _, hash := range hashes {
		mp[string(hash)] = p2p.Empty{}
	}

	// 获取最新100个区块的哈希
	blockStart := uint64(0)
	blockHashes := make([][]byte, 0)
	lastBlock := s.chain.GetLatestBlock()
	if lastBlock != nil {
		if lastBlock.Height >= defPerSummaryBlockHashNum {
			blockStart = lastBlock.Height - defPerSummaryBlockHashNum
		}
		for height := blockStart; height < lastBlock.Height; height++ {
			block, err := s.chain.GetBlock(height)
			if err != nil {
				break
			}
			blockHashes = append(blockHashes, block.Hash[:])
		}
	}

	msg := &p2p.Message{
		SrcId: s.Id,
		Content: &p2p.Message_PullReq{
			PullReq: &p2p.PullRequest{
				BlockStart:  blockStart,
				BlockHashes: blockHashes,
				TxHashes:    mp,
			},
		},
	}
	data, _ = msg.Marshal()
	return data
}

// LocalState 返回相关的本地状态信息。
// 参数：
//     summary: 远程节点的 pull 请求信息
// 返回值：
//     state: 状态数据
func (s *P2PServer) LocalState(data []byte) (state []byte) {
	msg := &p2p.Message{}
	if err := msg.Unmarshal(data); err != nil {
		log.Errorf("unmarshal pull request message failed: %v", err)
		return []byte{}
	}

	blocks := make([][]byte, 0)
	txs := make([][]byte, 0)
	req := msg.GetPullReq()

	// 获取远程节点没有的区块
	lastBlock := s.chain.GetLatestBlock()
	if lastBlock != nil {
		start := req.BlockStart + uint64(len(req.BlockHashes))
		height := req.BlockStart
		i := 0
		for height <= lastBlock.Height && i < len(req.BlockHashes) {
			block, err := s.chain.GetBlock(height)
			if err != nil {
				break
			}
			if !bytes.Equal(block.Hash[:], req.BlockHashes[i]) {
				// 高度为height的区块不相同，那么将之后的区块同步给远程节点
				start = height
				break
			}
			height++
			i++
		}
		num := defPerPullBlockNum
		for ; start <= lastBlock.Height; start++ {
			if num <= 0 {
				break
			}
			num--
			block, err := s.chain.GetBlock(height)
			if err != nil {
				break
			}
			blocks = append(blocks, block.Hash[:])
		}
	}

	// 获取远程节点没有的交易
	txHashes := req.TxHashes
	s.chain.GetTxPool().Traverse(func(_ uint64, tx *Transaction) bool {
		if _, ok := txHashes[string(tx.Hash[:])]; !ok {
			b, _ := tx.Marshal()
			txs = append(txs, b)
		}
		return true
	})

	msg = &p2p.Message{
		SrcId: s.Id,
		Content: &p2p.Message_PullResp{
			PullResp: &p2p.PullResponse{
				Blocks: blocks,
				Txs:    txs,
			},
		},
	}
	state, _ = msg.Marshal()
	return state
}

// MergeRemoteState 合并远程节点返回的状态信息。
// 参数：
//     state: 远程节点返回的状态信息
func (s *P2PServer) MergeRemoteState(data []byte) {
	msg := &p2p.Message{}
	if err := msg.Unmarshal(data); err != nil {
		log.Warnf("unmarshal message failed: %v", err)
		return
	}
	resp := msg.GetPullResp()

	// 同步区块
	for _, blockData := range resp.Blocks {
		block := Block{}
		if err := block.Unmarshal(blockData); err != nil {
			log.Warnf("unmarshal block failed: %v", err)
			break
		}
		// 注意：这里不一定就是最新的区块，可能是之前的区块
		if ok := s.chain.VerifyBlock(&block); !ok {
			log.Debug("verify block failed")
			return
		}

		if err := s.chain.AddBlock(&block); err != nil {
			log.Warnf("add block failed: %v", err)
			break
		}
	}

	// 同步交易
	for _, txData := range resp.Txs {
		tx := Transaction{}
		if err := tx.Unmarshal(txData); err != nil {
			log.Warnf("unmarshal transaction failed: %v", err)
			break
		}
		if err := s.chain.AddToTxPool(tx); err != nil {
			log.Warnf("add to tx pool failed: %v", err)
			break
		}
	}
}

func (s *P2PServer) startMining() {
	s.startMiningChan <- struct{}{}
}

func (s *P2PServer) stopMining() {
	s.stopMiningChan <- struct{}{}
}

// mining 挖矿
func (s *P2PServer) mining() {
	var (
		ctx    context.Context
		cancel context.CancelFunc = nil
	)

	for {
		select {
		case <-s.stopMiningChan:
			if cancel != nil {
				log.Debug("cancel mining...")
				cancel()
				cancel = nil
			}
		case <-s.startMiningChan:
			if cancel != nil {
				// 已经在挖矿
				break
			}

			log.Debug("start mining...")
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				if s.chain.GetLatestBlock() == nil {
					log.Info("create blockchain with genesis block...")
					s.chain.MineGenesisBlock(s.rewardAddress)
					cancel()
					cancel = nil
					return
				}

				for {
					err := s.chain.MineBlock(ctx, s.rewardAddress)
					if err != nil {
						log.Warn("mineBlock block failed: ", err)
						return
					}

					select {
					case <-ctx.Done():
						log.Debug("cancel mining...")
						// 取消挖矿
						return
					default:
					}

					log.Info("mining block success, height: ", s.chain.GetLatestBlock().Height)
				}
			}()
		}
	}
}

func (s *P2PServer) gossipNewTransaction(fee uint64, tx *Transaction) {
	txData, _ := tx.Marshal()
	msg := &p2p.Message{
		SrcId: s.Id,
		Content: &p2p.Message_NewTx{
			NewTx: &p2p.Transaction{
				Fee:  fee,
				Data: txData,
			},
		},
	}
	s.gossip(msg)
}

func (s *P2PServer) gossip(msg *p2p.Message) {
	data, _ := msg.Marshal()
	s.Lock()
	if s.broadcasts == nil {
		s.broadcasts = make([][]byte, 0)
	}
	s.broadcasts = append(s.broadcasts, data)
	s.Unlock()
}

func (s *P2PServer) dispatch() {
	for {
		select {
		case msg := <-s.accept:
			s.handleMessage(msg)
		case <-s.stop:
			return
		}
	}
}

func (s *P2PServer) handleMessage(msg *p2p.Message) {
	switch msg.Content.(type) {
	case *p2p.Message_NewTx:
		tx := Transaction{}
		if err := tx.Unmarshal(msg.GetNewTx().Data); err != nil {
			log.Warn(err)
			return
		}
		_ = s.chain.AddToTxPool(tx)
	case *p2p.Message_NewBlock:
		block := Block{}
		if err := block.Unmarshal(msg.GetNewBlock().Payload); err != nil {
			log.Warn(err)
			return
		}
		_ = s.chain.AddBlock(&block)
	}
}
