package main

import (
	"github.com/treeforest/easyblc"
	"github.com/treeforest/easyblc/config"
	log "github.com/treeforest/logger"
)

func main() {
	log.SetLevel(log.DEBUG)

	conf, err := config.Load()
	if err != nil {
		log.Fatal("load config failed")
	}
	// _ = os.RemoveAll("BLC")
	chain := blc.GetBlockChain(conf.DBPath)

	httpSrv := blc.NewHttpServer(conf.HttpServerPort, chain)
	go httpSrv.Run()

	//rpcSrv := blc.RunRpcSerer(conf.HttpServerPort+1, chain)
	//defer rpcSrv.Stop()

	peer := blc.NewServer(conf, chain)
	peer.Run()
}
