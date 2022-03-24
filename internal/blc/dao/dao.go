package dao

import (
	"errors"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	log "github.com/treeforest/logger"
	"os"
)

const (
	dbName             = "BLC"                       // 数据库名
	latestBlockHashKey = "__latest_block_hash_key__" // 最新区块哈希对应的key
)

func IsNotExistDB() bool {
	_, err := os.Stat(dbName)
	return os.IsNotExist(err)
}

// DAO 区块链存储对象
type DAO struct {
	*leveldb.DB
	latestBlockHash []byte // 最新区块哈希值
}

func New() *DAO {
	levelDB, err := leveldb.OpenFile(dbName, &opt.Options{})
	if err != nil {
		log.Fatalf("open leveldb [%s] error [%v]", dbName, err)
	}
	d := &DAO{DB: levelDB, latestBlockHash: nil}
	return d
}

// Load 加载已有的数据库
func Load() (d *DAO, err error) {
	d = New()
	err = d.loadLatestBlockHash()
	return
}

// loadLatestBlockHash load the latest block hash
func (d *DAO) loadLatestBlockHash() error {
	ro := &opt.ReadOptions{DontFillCache: false}
	value, err := d.DB.Get([]byte(latestBlockHashKey), ro)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return errors.New("not found latest block hash")
		}
		return fmt.Errorf("load latest block hash failed: %v", err)
	}
	d.latestBlockHash = value
	return nil
}

func (d *DAO) Close() error {
	return d.DB.Close()
}

func (d *DAO) GetLatestBlockHash() []byte {
	return d.latestBlockHash
}

func (d *DAO) GetBlock(hash []byte) ([]byte, error) {
	return d.DB.Get(hash, nil)
}

func (d *DAO) AddBlock(hash []byte, block []byte) error {
	wo := &opt.WriteOptions{Sync: true}
	if err := d.DB.Put(hash, block, wo); err != nil {
		return fmt.Errorf("add block failed: %v", err)
	}
	if err := d.DB.Put([]byte(latestBlockHashKey), hash, wo); err != nil {
		return fmt.Errorf("put latest block hash failed: %v", err)
	}
	d.latestBlockHash = hash
	return nil
}
