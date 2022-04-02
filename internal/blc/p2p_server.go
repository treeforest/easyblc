package blc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hashicorp/memberlist"
	"github.com/pborman/uuid"
	"github.com/treeforest/easyblc/internal/blc/config"
	"github.com/treeforest/easyblc/pkg/gob"
	log "github.com/treeforest/logger"
	"math/rand"
	"os"
	"sync"
	"time"
)

func init() {
	go func() {
		t := time.NewTicker(time.Second * 10)
		for {
			select {
			case <-t.C:
				// 改变随机种子，增加安全性
				rand.Seed(time.Now().UnixNano())
			}
		}
	}()
}

// State 节点在网络中的状态信息
type State struct {
	Height uint64
}

func NewState() *State {
	return &State{Height: 0}
}

func (state *State) Marshal() []byte {
	data, _ := json.Marshal(state)
	return data
}

func (state *State) Unmarshal(data []byte) error {
	return json.Unmarshal(data, state)
}

// NodeType 节点类型
type NodeType uint32

const (
	Full  NodeType = iota // 全节点
	Miner                 // 挖矿节点
	SPV                   // 轻节点
)

// OP 网络请求操作码
type OP uint32

const (
	OpGetBlock OP = iota + 1
	OpBlock
	OpGetHeader
	OpHeader
	OpGetState
	OpState
	OpPing
)

type MsgGetBlock struct {
	Start uint64
	End   uint64
}

type MsgBlock struct {
	Blocks []*Block
}

type MsgGetHeader struct {
	Start uint64
	End   uint64
}

type MsgHeader struct {
	Headers []*Block
}

type MsgState struct {
	State State
}

type P2PMessage struct {
	Op   OP
	From memberlist.Address // 来自谁
	Body []byte
}

func (m *P2PMessage) Marshal() []byte {
	data, _ := gob.Encode(m)
	return data
}

func (m *P2PMessage) Unmarshal(data []byte) error {
	return gob.Decode(data, m)
}

type Broadcast struct {
	memberlist.Broadcast
	msg    []byte
	notify chan struct{}
}

// Invalidates 检查当前发出的广播消息是否是无效的
func (b *Broadcast) Invalidates(mb memberlist.Broadcast) bool {
	// 始终有效
	return false
}

// Message 广播真实发送的消息
func (b *Broadcast) Message() []byte {
	return b.msg
}

// Finished 关闭广播消息，可用于通知消息发送结束
func (b *Broadcast) Finished() {
	if b.notify != nil {
		close(b.notify)
	}
}

// P2PServer 点对点服务
type P2PServer struct {
	memberlist    *memberlist.Memberlist
	broadcasts    *memberlist.TransmitLimitedQueue // 广播队列
	qu            [][]byte
	chain         *BlockChain                             // 区块链对象
	local         *State                                  // 本地状态
	localLocker   sync.RWMutex                            // 本地状态读写锁
	remote        *State                                  // 远端状态
	nodes         map[memberlist.Address]*memberlist.Node // 网络中的节点 地址->节点信息
	metadata      map[string]string                       // 节点元数据
	nodeType      NodeType                                // 节点类型
	rewardAddress string                                  // 挖矿奖励地址
	startMining   chan struct{}                           // 开始挖矿
	stopMining    chan struct{}                           // 停止挖矿
	msgs          chan *P2PMessage                        // 消息通道
}

func NewP2PServer() *P2PServer {
	conf, err := config.Load()
	if err != nil {
		log.Fatal("load config failed")
	}

	server := &P2PServer{
		qu:            make([][]byte, 0),
		chain:         GetBlockChain(conf.DBPath),
		local:         NewState(),
		localLocker:   sync.RWMutex{},
		remote:        NewState(),
		nodes:         map[memberlist.Address]*memberlist.Node{},
		metadata:      map[string]string{},
		nodeType:      NodeType(conf.Type),
		rewardAddress: conf.Address,
		startMining:   make(chan struct{}, 1),
		stopMining:    make(chan struct{}, 1),
		msgs:          make(chan *P2PMessage, 256),
	}

	server.init(conf)
	return server
}

func (s *P2PServer) Run() {
	go s.dispatchMessage(context.Background())
	time.Sleep(time.Millisecond * 50)

	//done := make(chan struct{}, 1)
	//go func() {
	//	t := time.NewTicker(time.Second)
	//	for {
	//		select {
	//		case <-t.C:
	//			s.localLocker.RLock()
	//			if s.local.Height >= s.remote.Height {
	//				s.localLocker.RUnlock()
	//				done <- struct{}{}
	//				return
	//			}
	//			const maxCount = 5000 // 一次同步区块的最大数量
	//			start, end := s.local.Height, s.remote.Height
	//			if s.remote.Height-s.local.Height > maxCount {
	//				end = s.local.Height + maxCount
	//			}
	//			s.localLocker.RUnlock()
	//
	//			m := MsgGetBlock{
	//				Start: start,
	//				End:   end,
	//			}
	//			data, _ := gob.Encode(m)
	//			log.Infof("send sync message, start:%d end:%d", start, end)
	//
	//			msg := &P2PMessage{Op: OpGetBlock, From: *s.memberlist.LocalNode(), Body: data}
	//			err := s.SendToRandNode(msg)
	//			if err != nil {
	//				log.Errorf("send to rand node failed: %v", err)
	//			}
	//		}
	//	}
	//}()
	//<-done
	//log.Info("数据同步完成")

	//go s.dispatchMessage(context.Background())
	//
	go s.sync(context.Background())
	if s.nodeType == Miner {
		go s.mining()
		s.startMining <- struct{}{} // 启动挖矿
	}

	select {}
}

func (s *P2PServer) init(conf *config.Config) {
	hostname, _ := os.Hostname()
	c := memberlist.DefaultLANConfig()
	c.UDPBufferSize = 2000
	c.Events = s
	c.Delegate = s
	c.BindPort = conf.Port // 端口
	c.Name = hostname + "-" + uuid.NewUUID().String()

	m, err := memberlist.Create(c)
	if err != nil {
		log.Fatal("memberlist create failed:", err)
	}

	if len(conf.Existing) != 0 {
		// 加入当前区块链网络
		existing := conf.Existing
		_, err = m.Join(existing)
		if err != nil {
			log.Fatalf("memberlist join failed, existing:%v err:%v", existing, err)
		}
		log.Info("join blockchain network success")
	}

	s.memberlist = m
	s.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			alive := s.memberlist.NumMembers()
			return alive
		}, // 集群中的节点数
		RetransmitMult: 3, // 发送失败时的重试次数
	}

	if s.chain.GetLatestBlock() != nil {
		s.local.Height = s.chain.GetLatestBlock().Height
	}

}

// mining 挖矿
func (s *P2PServer) mining() {
	// log.Info("start mining...")

	mineBlock := func(ctx context.Context) {
		for {
			s.localLocker.RLock()
			if s.local.Height < s.remote.Height {
				s.localLocker.RUnlock()
				return
			}
			s.localLocker.RUnlock()

			err := s.chain.MineBlock(ctx, s.rewardAddress)
			if err != nil {
				log.Warn("mineBlock block failed: ", err)
				return
			}

			t := time.NewTicker(time.Millisecond * 100)
			select {
			case <-ctx.Done():
				log.Debug("cancel mining...")
				// 取消挖矿
				return
			case <-t.C:
			}

			log.Info("mining block success, height: ", s.chain.GetLatestBlock().Height)
			// 挖到区块
			// 1、更新本地状态
			s.localLocker.Lock()
			s.local.Height = s.chain.GetLatestBlock().Height
			s.localLocker.Unlock()

			// 2、广播区块
			m := &MsgBlock{
				Blocks: []*Block{s.chain.GetLatestBlock()},
			}
			data, _ := gob.Encode(m)
			log.Debug(len(data))
			s.Gossip(&P2PMessage{
				Op:   OpBlock,
				From: s.FullAddress(),
				Body: data,
			})
		}
	}

	var (
		ctx    context.Context
		cancel context.CancelFunc = nil
	)

	for {
		select {
		case <-s.stopMining:
			if cancel != nil {
				cancel()
				cancel = nil
			}
		case <-s.startMining:
			if cancel != nil {
				// 已经在挖矿
				break
			}

			ctx, cancel = context.WithCancel(context.Background())
			go mineBlock(ctx)
		}
	}
}

// sync 同步区块
func (s *P2PServer) sync(ctx context.Context) {
	t := time.NewTicker(time.Millisecond * 500)
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.localLocker.RLock()
			if s.local.Height >= s.remote.Height {
				s.localLocker.RUnlock()
				if s.nodeType == Miner {
					// 区块继续挖矿
					s.startMining <- struct{}{}
					log.Debug("restart mining")
				}
				return
			}
			start, end := s.local.Height, s.remote.Height
			s.localLocker.RUnlock()

			const maxCount = 2000 // 一次同步区块的最大数量
			if end-start > maxCount {
				end = start + maxCount
			}

			// 取消挖矿，当同步了最新足区块后再开始挖矿
			if s.nodeType == Miner {
				s.stopMining <- struct{}{}
				log.Debug("stop mining")
			}

			var (
				data []byte
				op   OP
			)
			if s.nodeType == Miner || s.nodeType == Full {
				op = OpGetBlock
				m := MsgGetBlock{
					Start: start,
					End:   end,
				}
				data, _ = gob.Encode(m)
			} else if s.nodeType == SPV {
				op = OpGetHeader
				m := &MsgGetHeader{
					Start: start,
					End:   end,
				}
				data, _ = gob.Encode(m)
			} else {
				log.Fatal("illegal node type")
			}

			for i := 0; i < 3; i++ {
				err := s.SendToRandNode(&P2PMessage{
					Op:   op,
					From: s.FullAddress(),
					Body: data,
				})
				if err != nil {
					log.Errorf("send to rand node failed: %v", err)
				}
			}
		}
	}
}

func (s *P2PServer) dispatchMessage(ctx context.Context) {
	for {
		select {
		case msg := <-s.msgs:
			go s.handleMessage(msg)
		case <-ctx.Done():
			return
		}
	}
}

func (s *P2PServer) handleMessage(msg *P2PMessage) {
	if msg.From.Addr == s.FullAddress().Addr {
		return
	}

	// log.Infof("got msg -> op:%d from:%s", msg.Op, msg.From.Addr)

	switch msg.Op {
	case OpPing:
		log.Info("ping message")

	case OpGetHeader:
		// TODO

	case OpGetBlock:
		{
			if s.chain.GetLatestBlock() == nil {
				log.Debug("latest block is nil")
				return
			}

			var req MsgGetBlock
			err := gob.Decode(msg.Body, &req)
			if err != nil {
				log.Errorf("gob decode failed: %v", err)
				return
			}

			resp := MsgBlock{Blocks: make([]*Block, 0)}
			for height := req.Start; height <= req.End; height++ {
				if height <= s.chain.GetLatestBlock().Height {
					block, err := s.chain.GetBlock(height)
					if err != nil {
						log.Errorf("get block failed, height:%d err:%v", height, err)
						break
					}
					// log.Info("find block: ", block.Height)
					resp.Blocks = append(resp.Blocks, block)
				}
			}
			if len(resp.Blocks) == 0 {
				break
			}
			data, _ := gob.Encode(resp)
			log.Infof("begin send blocks [start:%d] [end:%d]...", resp.Blocks[0].Height, resp.Blocks[len(resp.Blocks)-1].Height)

			s.SendReliable(msg.From, &P2PMessage{
				Op:   OpBlock,
				From: s.FullAddress(),
				Body: data,
			})
		}

	case OpBlock:
		{
			var req = MsgBlock{Blocks: make([]*Block, 0)}
			err := gob.Decode(msg.Body, &req)
			if err != nil {
				log.Errorf("gob decode failed: %v", err)
				return
			}

			isSync := false
			s.localLocker.Lock()
			for _, block := range req.Blocks {
				if (s.chain.GetLatestBlock() == nil && block.Height == 0) ||
					block.Height == s.local.Height+1 {

					if !isSync {
						log.Debug("begin sync...")
						isSync = true
						if s.nodeType == Miner {
							log.Debug("stop mining...")
							s.stopMining <- struct{}{}
						}
					}

					// test
					//tmp, _ := json.MarshalIndent(block, "", "\t")
					//log.Debug(string(tmp))

					if err = s.chain.AddBlock(block); err != nil {
						log.Errorf("add block error: %v", err)
						break
					}

					log.Debugf("sync block, height: %d", block.Height)
					s.local.Height = block.Height
					//fmt.Print(".")
				}
			}
			s.localLocker.Unlock()

			if !isSync {
				return
			}

			//fmt.Println()
			log.Debug("sync end, local height:", s.local.Height)

			s.localLocker.RLock()
			if s.nodeType == Miner && s.local.Height >= s.remote.Height {
				s.startMining <- struct{}{}
			}
			s.localLocker.RUnlock()
		}

	default:

	}
}

// NodeMeta 节点元数据，元数据若是发生改变，则会广播至网络
func (s *P2PServer) NodeMeta(limit int) []byte {
	meta, _ := json.Marshal(s.metadata)
	if len(meta) > limit {
		log.Warn("metadata length is over limit")
	}
	return meta
}

// NotifyMsg gossip 消息的回调
func (s *P2PServer) NotifyMsg(data []byte) {
	if len(data) == 0 {
		log.Warn("invalid message")
		return
	}

	var msg P2PMessage
	if err := msg.Unmarshal(data); err != nil {
		log.Errorf("NotifyMsg unmarshal p2pMessage failed: %v", err)
		return
	}

	s.msgs <- &msg
}

// GetBroadcasts 当gossip定时事件触发，将回调该函数.注意：！！！最多只能发送limit-overhead的字节，
// 数据过多将自动丢弃数据
func (s *P2PServer) GetBroadcasts(overhead, limit int) [][]byte {
	gossipMsg := s.broadcasts.GetBroadcasts(overhead, limit)
	need := 0
	for i := 0; i < len(gossipMsg); i++ {
		need += len(gossipMsg[i])
	}
	if need != 0 {
		log.Debugf("overhead:%d limit:%d have:%d need:%d", overhead, limit, limit-overhead, need)
	}
	return gossipMsg
}

// LocalState 本地状态回调，当加入网络的时候，远端节点需要获取当前节点状态时调用；
// 之后远端节点通过定时操作，需要同步状态的时候会发起网络请求，将会回调该函数。
// 参数：
// 		join: 是否为刚加入网络，true:是， false：否
func (s *P2PServer) LocalState(join bool) []byte {
	s.localLocker.RLock()
	state := s.local.Marshal()
	s.localLocker.RUnlock()
	return state
}

// MergeRemoteState 更新状态
func (s *P2PServer) MergeRemoteState(data []byte, join bool) {
	if len(data) == 0 {
		return
	}

	state := NewState()
	if err := state.Unmarshal(data); err != nil {
		log.Fatal("unmarshal state failed: ", err)
	}
	if state.Height < s.remote.Height {
		return
	}
	// log.Info("sync remote state:", state.Height)
	_ = s.remote.Unmarshal(data)
	log.Debugf("merge remote height: %d", s.remote.Height)
}

func (s *P2PServer) NotifyJoin(node *memberlist.Node) {
	log.Debug("node join: ", node.Address())
	s.nodes[node.FullAddress()] = node
}

func (s *P2PServer) NotifyLeave(node *memberlist.Node) {
	log.Debug("node leave: ", node.Address())
	delete(s.nodes, node.FullAddress())
}

// NotifyUpdate 节点更新，通常会因为节点元数据的更新而回调该函数
func (s *P2PServer) NotifyUpdate(node *memberlist.Node) {
	log.Debug("node update: ", node.Address())
	s.nodes[node.FullAddress()] = node
}

func (s *P2PServer) SendToRandNode(msg *P2PMessage) error {
	num := len(s.nodes)
	if num == 0 {
		return errors.New("not found")
	}
	var node *memberlist.Node
	for _, n := range s.nodes {
		node = n
		break
	}
	log.Debug("send to ", node.Address())
	return s.memberlist.SendReliable(node, msg.Marshal())
}

func (s *P2PServer) SendReliable(to memberlist.Address, msg *P2PMessage) {
	node, ok := s.nodes[to]
	if !ok {
		log.Warn("not found node with address %s", to.String())
		return
	}
	err := s.memberlist.SendReliable(node, msg.Marshal())
	if err != nil {
		log.Errorf("send to address[%v] failed:%v", to.String(), err)
	}
}

// Gossip gossip 广播消息,紧紧用于广播最新区块
func (s *P2PServer) Gossip(msg *P2PMessage) {
	log.Debugf("gossip op:%v msglen:%d", msg.Op, len(msg.Marshal()))
	s.broadcasts.QueueBroadcast(&Broadcast{
		msg:    msg.Marshal(),
		notify: nil,
	})
}

func (s *P2PServer) LocalNode() *memberlist.Node {
	return s.memberlist.LocalNode()
}

func (s *P2PServer) FullAddress() memberlist.Address {
	return s.LocalNode().FullAddress()
}
