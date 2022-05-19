package blc

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log "github.com/treeforest/logger"
	"io/ioutil"
	"net/http"
)

type HttpServer struct {
	port  int
	chain *BlockChain
}

func NewHttpServer(port int, chain *BlockChain) *HttpServer {
	return &HttpServer{port: port, chain: chain}
}

func (s *HttpServer) Run() {
	r := gin.Default()

	r.GET("/height", s.handleGetHeight)
	r.GET("/blocks", s.handleGetBlocks)
	r.GET("/balance", s.handleGetBalance)
	r.GET("/utxo", s.handleGetUtxo)
	r.GET("/txPool", s.handleGetTxPool)
	r.POST("/tx", s.handlePostTx)

	err := r.Run(fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Fatal("http server run failed:", err)
	}
}

func (s *HttpServer) handleGetHeight(c *gin.Context) {
	if !s.checkEnvironment(c) {
		return
	}
	type Response struct {
		Height uint64 `json:"height"`
	}
	c.JSON(http.StatusOK, Response{Height: s.chain.GetLatestBlock().Height})
}

func (s *HttpServer) handleGetBlocks(c *gin.Context) {
	if !s.checkEnvironment(c) {
		return
	}

	type Request struct {
		Start uint64 `json:"start"`
		End   uint64 `json:"end"`
	}
	req := Request{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request obj error"})
		return
	}

	type Response struct {
		Blocks []Block `json:"blocks"`
	}
	resp := Response{Blocks: make([]Block, 0)}
	for height := req.Start; height <= req.End; height++ {
		b, err := s.chain.GetBlock(height)
		if err != nil {
			break
		}
		resp.Blocks = append(resp.Blocks, *b)
	}

	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) handleGetBalance(c *gin.Context) {
	if !s.checkEnvironment(c) {
		return
	}

	type Request struct {
		Address string `json:"address"`
	}
	req := Request{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request obj error"})
		return
	}

	type Response struct {
		Balance uint64 `json:"balance"`
	}
	resp := Response{Balance: s.chain.GetBalance(req.Address)}
	c.JSON(http.StatusOK, resp)
}

func (s *HttpServer) handleGetUtxo(c *gin.Context) {
	if !s.checkEnvironment(c) {
		return
	}

	type Request struct {
		Address string `json:"address"`
	}
	req := Request{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "request obj error"})
		return
	}

	type Utxo struct {
		TxHash [32]byte
		Index  int
		TxOut  TxOutput
	}
	utxoes := make([]Utxo, 0)
	s.chain.utxoSet.Traverse(func(txHash [32]byte, index int, output TxOutput) {
		utxoes = append(utxoes, Utxo{
			TxHash: txHash,
			Index:  index,
			TxOut:  output,
		})
	})

	c.JSON(http.StatusOK, utxoes)
}

func (s *HttpServer) handleGetTxPool(c *gin.Context) {
	c.JSON(http.StatusOK, s.chain.txPool)
}

func (s *HttpServer) handlePostTx(c *gin.Context) {
	if !s.checkEnvironment(c) {
		return
	}

	var tx Transaction
	data, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err = tx.Unmarshal(data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unmarshal failed"})
		return
	}

	if err = s.chain.AddToTxPool(tx); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func (s *HttpServer) checkEnvironment(c *gin.Context) bool {
	if s.chain.GetLatestBlock() == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "not exist blockchain"})
		return false
	}
	return true
}
