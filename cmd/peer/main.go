package main

import (
	"github.com/treeforest/easyblc"
	"github.com/treeforest/easyblc/config"
	log "github.com/treeforest/logger"
)

func main() {
	log.SetLevel(log.DEBUG)

	conf, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("load config error: +v", err)
	}

	chain := blc.GetBlockChain(conf.LevelDBPath)

	httpSrv := blc.NewHttpServer(conf.HttpServerPort, chain)
	go httpSrv.Run()

	rpcSrv := blc.RunRpcSerer(conf.RpcServerPort, chain)
	defer rpcSrv.Stop()

	peer := blc.NewServer(log.INFO, conf, chain)
	peer.Run()
}
