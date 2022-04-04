package client

import (
	"flag"
	log "github.com/treeforest/logger"
	"os"
)

func parseCommand(f *flag.FlagSet) bool {
	if err := f.Parse(os.Args[2:]); err != nil {
		log.Fatal("parse command failed!")
	}
	if f.Parsed() {
		return true
	}
	return false
}
