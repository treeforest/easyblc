package blc

import (
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/pb"
	"github.com/treeforest/easyblc/pkg/bloom"
	rpc "github.com/treeforest/easyblc/pkg/rpc"
	log "github.com/treeforest/logger"
	"net"
)

type RpcServer struct {
	server *rpc.Server
	chain  *BlockChain
	addr   string
}

func NewTcpServer(addr string, chain *BlockChain) *RpcServer {
	s := &RpcServer{
		server: rpc.NewServer(),
		chain:  chain,
		addr:   addr,
	}
	s.register()
	return s
}

func (s *RpcServer) register() {
	err := s.server.RegisterName("rpc", s)
	if err != nil {
		panic(fmt.Errorf("register failed: %v", err))
	}
}

func (s *RpcServer) Run() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("listen failed: %v", err)
	}
	s.server.Accept(lis)
	return nil
}

// SendTx 发送交易
func (s *RpcServer) SendTx(args pb.SendTxReq, reply *pb.SendTxReply) error {
	var tx Transaction
	if err := tx.Unmarshal(args.Tx); err != nil {
		log.Debugf("unmarshal tx data failed: %v", err)
		return err
	}
	// 放入交易池
	if err := s.chain.AddToTxPool(&tx); err != nil {
		log.Debugf("add to tx pool failed: %v", err)
		return err
	}

	reply.Status = true
	return nil
}

// QueryBalance 查询余额
func (s *RpcServer) QueryBalance(req pb.QueryReq, reply *pb.QueryReply) error {
	filter := bloom.New()
	if err := filter.GobDecode(req.BloomData); err != nil {
		log.Debugf("bloom filter decode failed: %v", err)
		return err
	}

	balances := make(map[string]uint64)
	s.chain.utxoSet.Traverse(func(txHash [32]byte, index int, output *TxOutput) {
		if filter.Test([]byte(output.Address)) {
			balances[output.Address] = s.chain.GetBalance(output.Address)
		}
	})

	reply.Body = balances
	return nil
}

// QueryTx 查询交易
func (s *RpcServer) QueryTx(req pb.QueryReq, reply *pb.QueryReply) error {
	filter := bloom.New()
	if err := filter.GobDecode(req.BloomData); err != nil {
		log.Debugf("bloom filter decode failed: %v", err)
		return err
	}

	txs := make([]*Transaction, 0)
	_ = s.chain.Traverse(func(block *Block) {
		for _, tx := range block.Transactions {
			if filter.Test(tx.Hash[:]) {
				txs = append(txs, tx)
			}
		}
	})

	reply.Body = txs
	return nil
}

// QueryUTXO 查询UTXO
func (s *RpcServer) QueryUTXO(req pb.QueryReq, reply *pb.QueryReply) error {
	filter := bloom.New()
	if err := filter.GobDecode(req.BloomData); err != nil {
		log.Debugf("bloom filter decode failed: %v", err)
		return err
	}

	utxoSet := NewUTXOSet()
	s.chain.utxoSet.Traverse(func(txHash [32]byte, index int, output *TxOutput) {
		if filter.Test([]byte(output.Address)) {
			utxoSet.Put(txHash, index, output)
		}
	})

	reply.Body = utxoSet
	return nil
}

// Height 区块高度
func (s *RpcServer) Height(args interface{}, reply *uint64) error {
	*reply = s.chain.latestBlock.Height
	return nil
}
