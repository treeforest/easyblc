package blc

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/hashicorp/memberlist"
	"github.com/pborman/uuid"
	"github.com/treeforest/easyblc/internal/blc/config"
	"github.com/treeforest/easyblc/pkg/gob"
	"github.com/treeforest/easyblc/pkg/graceful"
	log "github.com/treeforest/logger"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
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
				t.Reset(time.Second * time.Duration(rand.Int63n(10)))
			}
		}
	}()
}

// State 节点在网络中的状态信息
type State struct {
	Height          uint64 // 节点高度
	HasGenisisBlock bool   // 区块链状态，true：已有创世区块，false：无创世区块
	rw              sync.RWMutex
}

func NewState() *State {
	return &State{
		Height:          0,
		HasGenisisBlock: false,
		rw:              sync.RWMutex{},
	}
}

func (s *State) Marshal() []byte {
	s.rw.RLock()
	s.rw.RUnlock()
	data, err := gob.Encode(s)
	if err != nil {
		log.Errorf("state marshal failed: %v", err)
		return []byte{}
	}
	return data
}

func (s *State) Unmarshal(data []byte) error {
	s.rw.Lock()
	defer s.rw.Unlock()
	return gob.Decode(data, s)
}

func (s *State) Copy() *State {
	state := NewState()
	s.rw.RLock()
	state.Height = s.Height
	state.HasGenisisBlock = s.HasGenisisBlock
	s.rw.RUnlock()
	return state
}

func (s *State) GetHeight() uint64 {
	s.rw.RLock()
	defer s.rw.RUnlock()
	return s.Height
}

func (s *State) Set(height uint64, hasBlc bool) {
	s.rw.Lock()
	defer s.rw.Unlock()
	s.Height = height
	s.HasGenisisBlock = hasBlc
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
	memberlist      *memberlist.Memberlist
	broadcasts      *memberlist.TransmitLimitedQueue        // 广播队列
	chain           *BlockChain                             // 区块链对象
	local           *State                                  // 本地状态
	remote          *State                                  // 远端状态
	remoteHeartbeat int32                                   // 对远端状态的心跳检测
	nodes           map[memberlist.Address]*memberlist.Node // 网络中的节点 地址->节点信息
	metadata        map[string]string                       // 节点元数据
	nodeType        NodeType                                // 节点类型
	rewardAddress   string                                  // 挖矿奖励地址
	startMining     chan struct{}                           // 开始挖矿
	stopMining      chan struct{}                           // 停止挖矿
	msgs            chan *P2PMessage                        // 消息通道
	conf            *config.Config
}

func NewP2PServer() *P2PServer {
	conf, err := config.Load()
	if err != nil {
		log.Fatal("load config failed")
	}

	server := &P2PServer{
		chain:           GetBlockChain(conf.DBPath),
		local:           NewState(),
		remote:          NewState(),
		remoteHeartbeat: 0,
		nodes:           map[memberlist.Address]*memberlist.Node{},
		metadata:        map[string]string{},
		nodeType:        NodeType(conf.Type),
		rewardAddress:   conf.Address,
		startMining:     make(chan struct{}, 1),
		stopMining:      make(chan struct{}, 1),
		msgs:            make(chan *P2PMessage, 512),
		conf:            conf,
	}

	server.init(conf)
	return server
}

func (s *P2PServer) Run() {
	go s.dispatchMessage(context.Background())
	time.Sleep(time.Millisecond * 100)

	go s.remoteHeartbeatTrigger()
	go s.syncTrigger()
	if s.nodeType == Miner {
		go s.mining()
		// s.startMining <- struct{}{} // 启动挖矿
	}

	graceful.Stop(func() {
		log.Info("graceful stopping...")
		s.Close()
	})
}

func (s *P2PServer) Close() {
	if s.nodeType == Miner {
		s.stopMining <- struct{}{}
	}
	err := s.memberlist.Shutdown()
	if err != nil {
		log.Fatal("network shutdown error:", err)
	}
	s.chain.Close()
}

func (s *P2PServer) init(conf *config.Config) {
	hostname, _ := os.Hostname()
	c := memberlist.DefaultLANConfig()
	c.UDPBufferSize = 2000
	c.PushPullInterval = time.Second * 10 // 状态同步间隔时间
	c.Events = s
	c.Delegate = s
	c.BindPort = conf.Port // 端口
	c.Name = hostname + "-" + uuid.NewUUID().String()

	m, err := memberlist.Create(c)
	if err != nil {
		log.Fatal("memberlist create failed:", err)
	}

	if s.chain.GetLatestBlock() != nil {
		s.local.Set(s.chain.GetLatestBlock().Height, true)
		log.Info("local height: ", s.local.GetHeight())
	}

	if len(conf.Existing) != 0 {
		// 加入当前区块链网络
		existing := conf.Existing
		_, err = m.Join(existing)
		if err != nil {
			log.Errorf("join blockchain network failed, existing:%v err:%v", existing, err)
		} else {
			log.Info("join blockchain network success")
		}
	}

	s.memberlist = m
	s.broadcasts = &memberlist.TransmitLimitedQueue{
		NumNodes: func() int {
			alive := s.memberlist.NumMembers()
			return alive
		},                 // 集群中的节点数
		RetransmitMult: 3, // 发送失败时的重试次数
	}
}

// mining 挖矿
func (s *P2PServer) mining() {
	var (
		ctx    context.Context
		cancel context.CancelFunc = nil
	)

	for {
		select {
		case <-s.stopMining:
			if cancel != nil {
				log.Debug("cancel mining...")
				cancel()
				cancel = nil
			}
		case <-s.startMining:
			if cancel != nil {
				// 已经在挖矿
				break
			}

			log.Debug("start mining...")
			ctx, cancel = context.WithCancel(context.Background())
			go func() {
				if s.chain.GetLatestBlock() == nil {
					CreateBlockChainWithGenesisBlock(s.conf.DBPath, s.conf.Address)
					cancel()
					return
				}

				for {
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
					// 挖到区块,更新本地状态
					s.local.Set(s.chain.GetLatestBlock().Height, true)
				}
			}()
		}
	}
}

// remoteHeartbeatTrigger 心跳触发器
func (s *P2PServer) remoteHeartbeatTrigger() {
	t := time.NewTicker(time.Second * 1)
	for {
		select {
		case <-t.C:
			// log.Debug("remote heartbeat: ", atomic.LoadInt32(&s.remoteHeartbeat))
			if atomic.LoadInt32(&s.remoteHeartbeat) == 0 {
				log.Debug("remote heartbeat timeout, begin reset remote state...")
				s.remote.Set(0, false)
				break
			}
			atomic.AddInt32(&s.remoteHeartbeat, -1) // 心跳减一
		}
	}
}

// syncTrigger 同步触发器，用于同步区块，并控制挖矿的启动与暂停
func (s *P2PServer) syncTrigger() {
	ch := make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(time.Millisecond * 100)
		timer := time.NewTimer(time.Second)
		for {
			select {
			case <-timer.C:
				if s.local.HasGenisisBlock == true && s.nodeType == Miner {
					// 延迟启动挖矿
					s.startMining <- struct{}{}
				}
			case <-ticker.C:
				if s.remote.HasGenisisBlock == true {
					ch <- struct{}{}
					return
				}
			}
		}
	}()
	<-ch

	ticker := time.NewTicker(time.Millisecond * 500)
	for {
		select {
		case <-ticker.C:
			localState := s.local.Copy()
			remoteState := s.remote.Copy()

			if s.local.HasGenisisBlock {
				// 若是矿工，即使对方先挖出一个块，没关系，我自继续挖，争取做最长链
				if s.nodeType == Miner && localState.GetHeight()+1 >= remoteState.GetHeight() {
					// 矿工继续挖矿
					s.startMining <- struct{}{}
					ticker.Reset(time.Millisecond * 500)
					break
				}
				if localState.GetHeight() >= remoteState.GetHeight() {
					ticker.Reset(time.Millisecond * 500)
					break
				}
			}

			ticker.Reset(time.Millisecond * 300)
			if s.nodeType == Miner {
				// 取消挖矿，当同步了最新区块后再重新启动挖矿
				s.stopMining <- struct{}{}
			}

			// 判断需要同步的区块区间
			var start, end uint64 = 0, remoteState.GetHeight()
			if localState.GetHeight() > 100 {
				start = localState.GetHeight() - 100
			}
			const maxCount = 3000 // 一次同步区块的最大数量
			if end-start > maxCount {
				end = start + maxCount
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

func (s *P2PServer) dispatchMessage(ctx context.Context) {
	for {
		select {
		case msg := <-s.msgs:
			s.handleMessage(msg)
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
				block, err := s.chain.GetBlock(height)
				if err != nil {
					log.Warnf("get block failed, height:%d err:%v", height, err)
					break
				}
				// log.Info("find block: ", block.Height)
				resp.Blocks = append(resp.Blocks, block)
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

			log.Debug("receive blocks, len=", len(req.Blocks))
			isSync := false
			for _, block := range req.Blocks {
				//log.Debug(block.Height, s.local.GetHeight())
				if block.Height > s.local.GetHeight()+1 {
					return
				}
				// must update
				b, _ := s.chain.GetBlock(block.Height)
				if b != nil {
					if block.Hash == b.Hash {
						//log.Debug("same hash -> height:", block.Height)
						continue
					}
					err = s.chain.RemoveBlockFrom(block.Height)
					if err != nil {
						log.Errorf("remove block from %d failed: %v", block.Height, err)
						break
					}
					//log.Debug("remove block from: ", block.Height)
					if block.PreHash != b.PreHash {
						return
					}
				}

				//log.Warn(block.Height)
				if err = s.chain.AddBlock(block); err != nil {
					log.Warnf("add block error: %v", err)
					return
				}
				s.local.Set(block.Height, true)
				log.Infof("sync block, height: %d", block.Height)
				isSync = true
			}

			if !isSync {
				// 没有同步区块
				return
			}

			log.Debug("sync end, local height:", s.local.Height)
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
	return s.local.Copy().Marshal()
}

// MergeRemoteState 更新状态
func (s *P2PServer) MergeRemoteState(data []byte, join bool) {
	if len(data) == 0 {
		return
	}

	state := NewState()
	if err := state.Unmarshal(data); err != nil {
		log.Fatal("unmarshal remote failed: ", err)
	}

	atomic.SwapInt32(&s.remoteHeartbeat, 15) // 满血心跳15s（系统设置的状态同步时间为10s）
	if (state.GetHeight() < s.remote.GetHeight()) ||
		(state.GetHeight() == s.remote.GetHeight() && s.remote.HasGenisisBlock) {
		return
	}

	s.remote.Set(state.GetHeight(), state.HasGenisisBlock)
	log.Infof("update remote[height:%d state:%v], local[height:%d state:%v]",
		s.remote.GetHeight(), s.remote.HasGenisisBlock, s.local.Height, s.local.HasGenisisBlock)
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
		log.Warnf("not found node with address %s", to.String())
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
