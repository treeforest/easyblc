package blc

import (
	"bytes"
	"context"
	"github.com/google/uuid"
	"github.com/treeforest/easyblc/config"
	"github.com/treeforest/easyblc/graceful"
	pb "github.com/treeforest/easyblc/proto"
	"github.com/treeforest/gossip"
	log "github.com/treeforest/logger"
	"sync"
	"time"
)

const (
	// defPerGossipNum 每次广播的gossip消息数量
	defPerGossipNum = 20
	// defPerSummaryBlockHashCount 每次summary发送的区块哈希的数量
	defPerSummaryBlockHashNum = 200
	// defPerPullBlockNum 每次拉取的区块数量
	defPerPullBlockNum = 100
)

// Server 区块链节点服务。负责将交易和最新区块广播到区块链网络中，以及同步
// 其它节点的区块。广播采用的是gossip。
type Server struct {
	sync.RWMutex
	l               log.Logger
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

func NewServer(logLevel log.Level, conf *config.Config, chain *BlockChain) *Server {
	id := uuid.NewString()

	server := &Server{
		l:               log.NewStdLogger(log.WithPrefix("server"), log.WithLevel(logLevel)),
		Id:              id,
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

	gossipConf := gossip.DefaultConfig()
	gossipConf.Id = id
	gossipConf.Port = conf.Port
	gossipConf.Endpoint = conf.Endpoint
	gossipConf.BootstrapPeers = conf.BootstrapPeers
	gossipConf.Delegate = server
	gossipConf.PullInterval = time.Second * 4

	server.gossipSrv = gossip.New(gossipConf)

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
		s.l.Info("stopping...")
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
		s.l.Errorf("NotifyMsg unmarshal pbMessage failed: %v", err)
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

// GetPullRequest 返回 pull 请求时所携带的信息。
func (s *Server) GetPullRequest() (req []byte) {
	s.l.Debug("get pull request")
	txHashMap := s.getPullRequestTxsMap()
	blockStart, blockHashes := s.getPullRequestBlocksSlice()
	msg := &pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_PullReq{
			PullReq: &pb.PullRequest{
				BlockStart:  blockStart,
				BlockHashes: blockHashes,
				TxHashes:    txHashMap,
			},
		},
	}
	req, _ = msg.Marshal()
	return req
}

// ProcessPullRequest 处理pull请求，并返回结果。
func (s *Server) ProcessPullRequest(req []byte) (resp []byte) {
	s.l.Debug("process pull request")

	msg := &pb.Message{}
	if err := msg.Unmarshal(req); err != nil {
		s.l.Errorf("unmarshal pull request message failed: %v", err)
		return []byte{}
	}

	pullReq := msg.GetPullReq()
	if pullReq == nil {
		return []byte{}
	}

	// s.l.Debug("pull request: ", pullReq.String())
	blocks := s.getNeedSyncBlocks(pullReq.BlockStart, pullReq.BlockHashes)
	txs := s.getNeedSyncTxs(pullReq.TxHashes)

	s.l.Debugf("send local state, blocks:%d txHashes:%d", len(blocks), len(txs))

	msg = &pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_PullResp{
			PullResp: &pb.PullResponse{
				Blocks: blocks,
				Txs:    txs,
			},
		},
	}
	resp, _ = msg.Marshal()
	return resp
}

// ProcessPullResponse 合并远程节点返回的状态信息。
func (s *Server) ProcessPullResponse(resp []byte) {
	s.l.Debug("process pull response")

	msg := &pb.Message{}
	if err := msg.Unmarshal(resp); err != nil {
		s.l.Warnf("unmarshal message failed: %v", err)
		return
	}

	pullResp := msg.GetPullResp()
	if pullResp == nil {
		return
	}

	// 同步区块
	s.syncBlocks(pullResp.Blocks)

	// 同步交易
	s.syncTxs(pullResp.Txs)
}

// getPullRequestBlocksSlice 获得 pull request 需要发送的区块哈希切片
func (s *Server) getPullRequestBlocksSlice() (uint64, [][]byte) {
	blockStart := uint64(0)
	blockHashes := make([][]byte, 0)
	lastBlock := s.chain.GetLatestBlock()
	if lastBlock != nil {
		if lastBlock.Height >= defPerSummaryBlockHashNum {
			blockStart = lastBlock.Height - defPerSummaryBlockHashNum
		}
		for height := blockStart; height <= lastBlock.Height; height++ {
			hash, err := s.chain.GetBlockHash(height)
			if err != nil {
				break
			}
			blockHashes = append(blockHashes, hash)
		}
	}
	return blockStart, blockHashes
}

// getPullRequestTxsMap 获得 pull request 需要发送的交易哈希信息映射表
func (s *Server) getPullRequestTxsMap() map[string]pb.Value {
	// 获取当前交易池中所有交易的hash
	hashes := s.chain.GetTxPool().TxHashes()
	mp := make(map[string]pb.Value, len(hashes))
	for _, hash := range hashes {
		mp[string(hash)] = pb.Value{}
	}
	return mp
}

// getNeedSyncTxs 获得需要同步的交易
func (s *Server) getNeedSyncTxs(remotePeerTxHashes map[string]pb.Value) [][]byte {
	txs := make([][]byte, 0)
	s.chain.GetTxPool().Traverse(func(_ uint64, tx *Transaction) bool {
		if _, ok := remotePeerTxHashes[string(tx.Hash[:])]; !ok {
			b, _ := tx.Marshal()
			txs = append(txs, b)
		}
		return true
	})
	return txs
}

// getNeedSyncBlocks 获得需要同步的区块
func (s *Server) getNeedSyncBlocks(blockStart uint64, remotePeerBlockHashes [][]byte) [][]byte {
	blocks := make([][]byte, 0)

	// 获取远程节点没有的区块
	lastBlock := s.chain.GetLatestBlock()
	if lastBlock != nil {
		// 默认从最后一个区块开始同步
		syncBlockStartHeight := blockStart + uint64(len(remotePeerBlockHashes))

		i := 0
		for startHeight := blockStart; startHeight <= lastBlock.Height && i < len(remotePeerBlockHashes); {
			block, err := s.chain.GetBlock(startHeight)
			if err != nil {
				break
			}
			if !bytes.Equal(block.Hash[:], remotePeerBlockHashes[i]) {
				// 高度为height的区块不相同，那么将之后的区块同步给远程节点
				syncBlockStartHeight = startHeight
				break
			}
			startHeight++
			i++
		}

		num := defPerPullBlockNum
		for height := syncBlockStartHeight; height <= lastBlock.Height; height++ {
			if num <= 0 {
				break
			}
			num--
			block, err := s.chain.GetBlock(height)
			if err != nil {
				break
			}
			bData, _ := block.Marshal()
			blocks = append(blocks, bData)
		}
	}

	return blocks
}

// syncTxs 同步远程的交易
func (s *Server) syncTxs(txDataSlice [][]byte) {
	var err error
	for _, txData := range txDataSlice {
		tx := Transaction{}
		if err = tx.Unmarshal(txData); err != nil {
			s.l.Warnf("unmarshal transaction failed: %v", err)
			break
		}
		if err = s.chain.AddToTxPool(tx); err != nil {
			s.l.Warnf("add to tx pool failed: %v", err)
			break
		}
		s.l.Infof("sync transaction hash: %s", tx.Hash)
	}
}

// syncBlocks 同步远程的区块
func (s *Server) syncBlocks(blockDataSlice [][]byte) {
	blocks := make([]*Block, 0)
	var err error

	// 1. 检查接收到的所有区块，验证合法性（工作量证明）
	for i, blockData := range blockDataSlice {
		block := Block{}
		if err = block.Unmarshal(blockData); err != nil {
			s.l.Warnf("unmarshal block failed: %v", err)
			break
		}
		var preBlock *Block
		if i == 0 {
			if s.chain.GetLatestBlock() == nil || block.Height == 0 {
				preBlock = nil
				s.l.Debug("preBlock: null")
			} else {
				preBlock, err = s.chain.GetBlock(block.Height - 1)
				if err != nil {
					s.l.Debugf("get block failed, height:%d err:%v", block.Height-1, err)
					return
				}
				s.l.Debug("preBlock: ", preBlock.Height)
			}
		} else {
			preBlock = blocks[i-1]
			s.l.Debug("preBlock: ", preBlock.Height)
		}
		if ok := s.chain.VerifyBlock(preBlock, &block); !ok {
			s.l.Warnf("verify block %d failed", block.Height)
			return
		}
		blocks = append(blocks, &block)
	}

	if len(blocks) == 0 {
		return
	}

	if s.chain.GetLatestBlock() == nil && blocks[0].Height != 0 {
		// 没有初始区块，且同步的区块不是初始区块（不符合同步标准）
		return
	}
	if s.chain.GetLatestBlock() != nil && blocks[len(blocks)-1].Height < s.chain.GetLatestBlock().Height {
		// 有初始区块，且同步的区块的高度低于当前区块高度（不符合同步标准）
		return
	}

	// 开始同步区块
	s.stopMining()
	defer s.startMining()

	if err = s.chain.RemoveBlockFrom(blocks[0].Height); err != nil {
		s.l.Errorf("remove block from %d failed: %+v", blocks[0].Height, err)
		return
	}

	for _, block := range blocks {
		if err = s.chain.AddBlock(block); err != nil {
			s.l.Warnf("add block failed: %v", err)
			break
		}
		log.Infof("sync block height: %d", block.Height)
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
				s.l.Info("stop mining")
				cancel()
				<-done
				cancel = nil
			}
		case <-s.startMiningChan:
			if cancel != nil {
				// 正在挖矿
				break
			}

			s.l.Info("start mining...")
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				defer func() {
					done <- struct{}{}
				}()

				if s.chain.GetLatestBlock() == nil {
					s.l.Info("create blockchain with genesis block...")
					s.chain.MineGenesisBlock(ctx, s.rewardAddress)
					select {
					case <-ctx.Done():
						s.l.Debug("cancel mining...")
						// 取消挖矿
						return
					default:
					}
					s.l.Info("mining block success, height: 0")
					s.gossipBlock(s.chain.GetLatestBlock())
				}

				for {
					err := s.chain.MineBlock(ctx, s.rewardAddress)
					if err != nil {
						s.l.Warn("mineBlock block failed: ", err)
						return
					}

					select {
					case <-ctx.Done():
						s.l.Debug("cancel mining...")
						// 取消挖矿
						return
					default:
					}

					s.l.Info("mining block success, height: ", s.chain.GetLatestBlock().Height)
					s.gossipBlock(s.chain.GetLatestBlock())
				}
			}()
		}
	}
}

func (s *Server) gossipBlock(block *Block) {
	s.l.Debug("gossip new block, height=", block.Height)
	bData, _ := block.Marshal()
	s.gossip(&pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_Block{
			Block: &pb.Block{
				Payload: bData,
			},
		},
	})
}

func (s *Server) gossipTransaction(fee uint64, tx *Transaction) {
	s.l.Debugf("gossip new tx, fee=%d hash=%s", fee, tx.Hash)
	txData, _ := tx.Marshal()
	s.gossip(&pb.Message{
		SrcId: s.Id,
		Content: &pb.Message_Tx{
			Tx: &pb.TxMessage{
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
	s.l.Debug("process tx message: ", *msg.GetTx())

	tx := Transaction{}
	if err := tx.Unmarshal(msg.GetTx().Data); err != nil {
		s.l.Warn(err)
		return
	}
	if err := s.chain.AddToTxPool(tx); err == nil {
		s.l.Debug("add tx[fee:%d hash:%s] to txPool", msg.GetTx().Fee, tx.Hash)
	}
}

func (s *Server) processBlockMessage(msg *pb.Message) {
	s.l.Debug("process block message: ", *msg.GetBlock())

	block := Block{}
	if err := block.Unmarshal(msg.GetBlock().Payload); err != nil {
		s.l.Warn(err)
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

	if err = s.chain.AddBlock(&block); err == nil {
		s.l.Debugf("add block, height: %d", block.Height)
	}
}
