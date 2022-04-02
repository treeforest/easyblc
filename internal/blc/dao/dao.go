package dao

import (
	"errors"
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	log "github.com/treeforest/logger"
	"os"
	"path/filepath"
	"strconv"
)

const (
	dbName             = "BLC"                       // 数据库名
	latestBlockHashKey = "__latest_block_hash_key__" // 最新区块哈希对应的key
	heightPrefix       = "__block_height__"
)

func IsNotExistDB(path string) bool {
	_, err := os.Stat(filepath.Join(path, dbName))
	return os.IsNotExist(err)
}

// DAO 区块链存储对象
type DAO struct {
	*leveldb.DB
	latestBlockHash []byte // 最新区块哈希值
}

func New(dbPath string) *DAO {
	levelDB, err := leveldb.OpenFile(filepath.Join(dbPath, dbName), &opt.Options{})
	if err != nil {
		log.Fatalf("open leveldb [%s] error [%v]", dbName, err)
	}
	d := &DAO{DB: levelDB, latestBlockHash: nil}
	return d
}

// Load 加载已有的数据库
func Load(dbPath string) (o *DAO, err error) {
	o = New(dbPath)
	err = o.loadLatestBlockHash()
	return
}

// loadLatestBlockHash load the latest block hash
func (o *DAO) loadLatestBlockHash() error {
	ro := &opt.ReadOptions{DontFillCache: false}
	value, err := o.DB.Get([]byte(latestBlockHashKey), ro)
	if err != nil {
		if err == leveldb.ErrNotFound {
			return errors.New("not found latest block hash")
		}
		return fmt.Errorf("load latest block hash failed: %v", err)
	}
	o.latestBlockHash = value
	return nil
}

func (o *DAO) Close() error {
	return o.DB.Close()
}

func (o *DAO) GetLatestBlockHash() []byte {
	return o.latestBlockHash
}

func (o *DAO) GetBlock(hash []byte) ([]byte, error) {
	return o.DB.Get(hash, nil)
}

func (o *DAO) GetBlockByHeight(height uint64) ([]byte, error) {
	heightKey := heightPrefix + strconv.FormatUint(height, 16)
	hash, err := o.DB.Get([]byte(heightKey), nil)
	if err != nil {
		return nil, fmt.Errorf("get block hash failed:%v", err)
	}
	return o.DB.Get(hash, nil)
}

func (o *DAO) AddBlock(height uint64, hash [32]byte, block []byte) error {
	// 开启事务
	trans, err := o.DB.OpenTransaction()
	if err != nil {
		return fmt.Errorf("open transaction failed: %v", err)
	}
	defer func() {
		if err != nil {
			// 事务提交失败，销毁事务
			trans.Discard()
		}
	}()

	wo := &opt.WriteOptions{Sync: true}

	err = trans.Put(hash[:], block, wo)
	if err != nil {
		return fmt.Errorf("intsert block failed: %v", err)
	}

	err = trans.Put([]byte(latestBlockHashKey), hash[:], wo)
	if err != nil {
		return fmt.Errorf("update latest block hash failed: %v", err)
	}

	heightKey := heightPrefix + strconv.FormatUint(height, 16)
	err = trans.Put([]byte(heightKey), hash[:], wo)
	if err != nil {
		return fmt.Errorf("intsert block height-hash failed: %v", err)
	}

	err = trans.Commit()
	if err != nil {
		return fmt.Errorf("add block failed: %v", err)
	}

	o.latestBlockHash = hash[:]
	return nil
}
