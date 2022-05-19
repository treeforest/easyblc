package blc

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/treeforest/easyblc/config"
	"github.com/treeforest/easyblc/graceful"
	pb "github.com/treeforest/easyblc/pb/server"
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
	// defPerPullBlockNum 每次拉取的区块数量
	defPerPullBlockNum = 50
)

// Server 区块链节点服务。负责将交易和最新区块广播到区块链网络中，以及同步
// 其它节点的区块。广播采用的是gossip。
type Server struct {
	sync.RWMutex
	gossipSrv       gossip.Gossip
	Id              string
	broadcasts      [][]byte          // 广播队列
	chain           *BlockChain       // 区块链对象
	metadata        map[string]string // 节点元数据
	nodeType        pb.NodeType       // 节点类型
	rewardAddress   string            // 挖矿奖励地址
	startMiningChan chan struct{}     // 开始挖矿
	stopMiningChan  chan struct{}     // 停止挖矿
	accept          chan *pb.Message  // 消息通道
	conf            *config.Config
	stop            chan struct{}
	stopOnce        sync.Once
}

func NewServer(conf *config.Config, chain *BlockChain) *Server {
	id := uuid.NewString()
	gossipConf := gossip.DefaultConfig()
	gossipConf.Id = id
	gossipConf.Port = conf.Port
	gossipConf.Endpoint = conf.Endpoint
	gossipConf.BootstrapPeers = conf.BootstrapPeers

	server := &Server{
		Id:              id,
		gossipSrv:       gossip.New(gossipConf),
		broadcasts:      make([][]byte, 1024),
		chain:           chain,
		metadata:        map[string]string{},
		nodeType:        pb.NodeType(conf.Type),
		rewardAddress:   conf.RewardAddress,
		startMiningChan: make(chan struct{}, 1),
		stopMiningChan:  make(chan struct{}, 1),
		accept:          make(chan *pb.Message, 256),
		conf:            conf,
		stop:            make(chan struct{}, 1),
	}

	server.chain.GetTxPool().SetPutCallback(server.gossipTransaction)

	return server
}

func (s *Server) Run() {
	go s.dispatch()
	time.Sleep(time.Millisecond * 100)

	if s.nodeType == pb.NodeType_Miner {
		go s.mining()
		s.startMining()
	}

	graceful.Stop(func() {
		log.Info("graceful stopping...")
		s.Stop()
	})
}

func (s *Server) Stop() {
	s.stopOnce.Do(func() {
		if s.nodeType == pb.NodeType_Miner {
			s.stopMining()
		}
		close(s.stop)
		s.gossipSrv.Stop()
		s.chain.Close()
	})
}

// NotifyMsg 处理用户数据
// 参数：
// 		msg: 用户数据
func (s *Server) NotifyMsg(data []byte) {
	if data == nil || len(data) == 0 {
		return
	}
	msg := &pb.Message{}
	if err := msg.Unmarshal(data); err != nil {
		log.Errorf("NotifyMsg unmarshal pbMessage failed: %v", err)
		return
	}
	s.accept <- msg
}

// GetBroadcasts 返回需要进行gossip广播的数据。
// 返回值：
// 	   data: 待广播的消息
func (s *Server) GetBroadcasts() (data [][]byte) {
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
func (s *Server) Summary() (data []byte) {
	// 获取当前交易池中所有交易的hash
	hashes := s.chain.GetTxPool().TxHashes()
	mp := make(map[string]pb.Empty, len(hashes))
	for _, hash := range hashes {
		mp[string(hash)] = pb.Empty{}
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

	msg := &pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_PullReq{
			PullReq: &pb.PullRequest{
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
func (s *Server) LocalState(data []byte) (state []byte) {
	msg := &pb.Message{}
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

	msg = &pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_PullResp{
			PullResp: &pb.PullResponse{
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
func (s *Server) MergeRemoteState(data []byte) {
	msg := &pb.Message{}
	if err := msg.Unmarshal(data); err != nil {
		log.Warnf("unmarshal message failed: %v", err)
		return
	}

	resp := msg.GetPullResp()
	blocks := make([]*Block, 0)
	var err error

	// 1. 检查接收到的所有区块，验证合法性（工作量证明）
	for i, blockData := range resp.Blocks {
		block := Block{}
		if err = block.Unmarshal(blockData); err != nil {
			log.Warnf("unmarshal block failed: %v", err)
			break
		}
		var preBlock *Block
		if i == 0 {
			preBlock, err = s.chain.GetBlock(block.Height - 1)
			if err != nil {
				return
			}
		} else {
			preBlock = blocks[i-1]
		}
		if ok := s.chain.VerifyBlock(preBlock, &block); !ok {
			log.Debug("verify block failed")
			return
		}
		blocks = append(blocks, &block)
	}

	defer s.startMining()

	if len(blocks) > 0 && blocks[len(blocks)-1].Height > s.chain.GetLatestBlock().Height {
		// 开始同步区块
		s.stopMining()

		if err = s.chain.RemoveBlockFrom(blocks[0].Height); err != nil {
			log.Errorf("remove block from %d failed: %+v", blocks[0].Height, err)
			return
		}

		for _, block := range blocks {
			if err = s.chain.AddBlock(block); err != nil {
				log.Warnf("add block failed: %v", err)
				break
			}
		}
	}

	// 同步交易
	for _, txData := range resp.Txs {
		tx := Transaction{}
		if err = tx.Unmarshal(txData); err != nil {
			log.Warnf("unmarshal transaction failed: %v", err)
			break
		}
		if err = s.chain.AddToTxPool(tx); err != nil {
			log.Warnf("add to tx pool failed: %v", err)
			break
		}
	}
}

func (s *Server) startMining() {
	if s.nodeType == pb.NodeType_Miner {
		s.startMiningChan <- struct{}{}
	}
}

func (s *Server) stopMining() {
	if s.nodeType == pb.NodeType_Miner {
		s.stopMiningChan <- struct{}{}
	}
}

// mining 挖矿
func (s *Server) mining() {
	var ctx context.Context
	var cancel context.CancelFunc = nil
	done := make(chan struct{}, 1)

	for {
		select {
		case <-s.stopMiningChan:
			if cancel != nil {
				log.Debug("cancel mining...")
				cancel()
				<-done
				cancel = nil
			}
		case <-s.startMiningChan:
			if cancel != nil {
				// 正在挖矿
				break
			}

			log.Debug("start mining...")
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				defer func() {
					done <- struct{}{}
				}()

				if s.chain.GetLatestBlock() == nil {
					log.Info("create blockchain with genesis block...")
					s.chain.MineGenesisBlock(ctx, s.rewardAddress)
					select {
					case <-ctx.Done():
						log.Debug("cancel mining...")
						// 取消挖矿
						return
					default:
					}
					log.Info("mining block success, height: 0")
					s.gossipBlock(s.chain.GetLatestBlock())
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
					s.gossipBlock(s.chain.GetLatestBlock())
				}
			}()
		}
	}
}

func (s *Server) gossipBlock(block *Block) {
	bData, _ := block.Marshal()
	s.gossip(&pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_Block{
			Block: &pb.Envelope{
				Payload: bData,
			},
		},
	})
}

func (s *Server) gossipTransaction(fee uint64, tx *Transaction) {
	txData, _ := tx.Marshal()
	s.gossip(&pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_Tx{
			Tx: &pb.Transaction{
				Fee:  fee,
				Data: txData,
			},
		},
	})
}

func (s *Server) gossip(msg *pb.Message) {
	data, _ := msg.Marshal()
	s.Lock()
	if s.broadcasts == nil {
		s.broadcasts = make([][]byte, 0)
	}
	s.broadcasts = append(s.broadcasts, data)
	s.Unlock()
}

func (s *Server) dispatch() {
	for {
		select {
		case <-s.stop:
			return
		case msg := <-s.accept:
			switch msg.Content.(type) {
			case *pb.Message_Tx:
				s.processTxMessage(msg)
			case *pb.Message_Block:
				s.processBlockMessage(msg)
			}
		}
	}
}

func (s *Server) processTxMessage(msg *pb.Message) {
	tx := Transaction{}
	if err := tx.Unmarshal(msg.GetTx().Data); err != nil {
		log.Warn(err)
		return
	}
	_ = s.chain.AddToTxPool(tx)
}

func (s *Server) processBlockMessage(msg *pb.Message) {
	block := Block{}
	if err := block.Unmarshal(msg.GetBlock().Payload); err != nil {
		log.Warn(err)
		return
	}

	preBlock, err := s.chain.GetBlock(block.Height - 1)
	if err != nil {
		return
	}
	if ok := s.chain.VerifyBlock(preBlock, &block); !ok {
		return
	}

	s.stopMining()
	defer s.startMining()

	_ = s.chain.AddBlock(&block)
}
