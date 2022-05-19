package main

import (
	"github.com/treeforest/easyblc/internal/blc"
	"github.com/treeforest/easyblc/internal/blc/config"
	log "github.com/treeforest/logger"
)

func main() {
	conf, err := config.Load()
	if err != nil {
		log.Fatal("load config failed")
	}
	// _ = os.RemoveAll("BLC")
	chain := blc.GetBlockChain(conf.DBPath)

	server := blc.NewHttpServer(conf.HttpServerPort, chain)
	go server.Run()

	node := blc.NewServer(conf, chain)
	node.Run()
}
