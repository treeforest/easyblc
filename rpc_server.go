package blc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	pb "github.com/treeforest/easyblc/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func RunRpcSerer(port int, chain *BlockChain) *rpcServer {
	if port < 0 {
		panic("port < 0")
	}

	gSrv := grpc.NewServer(grpc.Creds(insecure.NewCredentials()))
	s := &rpcServer{
		chain: chain,
		gSrv:  gSrv,
		wg:    sync.WaitGroup{},
	}

	pb.RegisterApiServer(gSrv, s)

	lis, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", port))
	if err != nil {
		panic(err)
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		_ = gSrv.Serve(lis)
	}()

	return s
}

type rpcServer struct {
	chain    *BlockChain
	gSrv     *grpc.Server
	wg       sync.WaitGroup
	stopOnce sync.Once
}

func (s *rpcServer) Stop() {
	s.stopOnce.Do(func() {
		s.gSrv.Stop()
		s.wg.Done()
	})
}

func (s *rpcServer) GetHeight(ctx context.Context, req *pb.Empty) (*pb.GetHeightResp, error) {
	if s.chain.GetLatestBlock() == nil {
		return nil, errors.New("no block")
	}
	return &pb.GetHeightResp{Height: s.chain.GetLatestBlock().Height}, nil
}

func (s *rpcServer) GetBlocks(ctx context.Context, req *pb.GetBlocksReq) (*pb.GetBlocksResp, error) {
	blocks := make([]Block, 0)
	for height := req.StartHeight; height <= req.EndHeight; height++ {
		b, err := s.chain.GetBlock(height)
		if err != nil {
			break
		}
		blocks = append(blocks, *b)
	}
	data, _ := json.Marshal(blocks)
	return &pb.GetBlocksResp{Blocks: data}, nil
}

func (s *rpcServer) GetBalance(ctx context.Context, req *pb.GetBalanceReq) (*pb.GetBalanceResp, error) {
	return &pb.GetBalanceResp{Balance: s.chain.GetBalance(req.Address)}, nil
}

func (s *rpcServer) GetUTXO(ctx context.Context, req *pb.GetUTXOReq) (*pb.GetUTXOResp, error) {
	utxoes := make([]pb.UTXO, 0)
	s.chain.utxoSet.Traverse(func(txHash [32]byte, index int, output TxOutput) {
		utxoes = append(utxoes, pb.UTXO{
			TxHash: txHash[:],
			Index:  int32(index),
			TxOut: pb.TxOutput{
				Value:        output.Value,
				Address:      output.Address,
				ScriptPubKey: output.ScriptPubKey,
			},
		})
	})
	return &pb.GetUTXOResp{Utxos: utxoes}, nil
}

func (s *rpcServer) GetTxPool(context.Context, *pb.Empty) (*pb.GetTxPoolResp, error) {
	pool, _ := s.chain.GetTxPool().Marshal()
	return &pb.GetTxPoolResp{TxPool: pool}, nil
}

func (s *rpcServer) PostTx(ctx context.Context, req *pb.PostTxReq) (*pb.Empty, error) {
	hash, err := BytesToByte32(req.Tx.Hash)
	if err != nil {
		return &pb.Empty{}, errors.New("format error")
	}

	if len(req.Tx.Ins) == 0 || len(req.Tx.Outs) == 0 {
		return &pb.Empty{}, errors.New("format error")
	}

	if time.Now().UnixNano() > req.Tx.Timestamp {
		return &pb.Empty{}, errors.New("timestamp error")
	}

	vins := make([]TxInput, 0)
	vouts := make([]TxOutput, 0)

	for _, in := range req.Tx.Ins {
		txid, err := BytesToByte32(in.TxId)
		if err != nil {
			return &pb.Empty{}, errors.New("format error")
		}
		vins = append(vins, TxInput{
			TxId:             txid,
			Vout:             in.Vout,
			ScriptSig:        in.ScriptSig,
			CoinbaseDataSize: int(in.CoinbaseDataSize),
			CoinbaseData:     in.CoinbaseData,
		})
	}
	for _, out := range req.Tx.Outs {
		vouts = append(vouts, TxOutput{
			Value:        out.Value,
			Address:      out.Address,
			ScriptPubKey: out.ScriptPubKey,
		})
	}

	tx := Transaction{
		Hash:  hash,
		Vins:  vins,
		Vouts: vouts,
		Time:  req.Tx.Timestamp,
	}

	err = s.chain.AddToTxPool(tx)
	return &pb.Empty{}, err
}
