package main

import "github.com/treeforest/easyblc"

func main() {
	cli := blc.NewRpcClient("")
	cli.Run()
}
