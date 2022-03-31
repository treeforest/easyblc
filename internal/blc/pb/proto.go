package pb

type SyncBlockRequest struct {
	Block []byte
}
type SyncBlockReply struct {
}

// --- 交换 ”库存清单“

type GetBlockRequest struct {
	Numbers []uint64 // 区块号
}
type GetBlockReply struct {
	Blocks [][]byte
}

// --- 获取区块头

type GetHeadersRequest struct {
	Numbers []uint64 // 区块号
}
type GetHeadersReply struct {
	Headers [][]byte
}

// --- 向连接的节点广播自己的节点地址

type AddrRequest struct {
	Addr string
}
type AddrResponse struct {
}

// --- 向连接的节点获取其邻居节点

type GetAddrRequest struct {
	Self string // 自己的地址
}
type GetAddrReply struct {
	Addrs []string // 地址列表
}

type HeartbeatRequest struct {
	Self string
}
type HeartbeatReply struct {
}
