package main

import (
	"github.com/treeforest/easyblc/internal/client"
)

func main() {
	cmd := client.New("../leveldb/")
	cmd.Run()
}
