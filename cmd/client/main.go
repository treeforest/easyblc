package main

import "github.com/treeforest/easyblc/internal/client"

func main() {
	cli := client.NewHttpClient()
	cli.Run()
}
