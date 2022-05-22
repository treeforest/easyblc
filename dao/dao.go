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
	dbName               = "BLC"                       // 数据库名
	latestBlockHashKey   = "__latest_block_hash_key__" // 最新区块哈希对应的key
	latestBlockHeightKey = "__latest_block_height_key__"
	heightPrefix         = "__block_height__"
)

func IsNotExistDB(path string) bool {
	_, err := os.Stat(filepath.Join(path, dbName))
	return os.IsNotExist(err)
}

// DAO 区块链存储对象
type DAO struct {
	*leveldb.DB
	latestBlockHash   []byte // 最新区块哈希值
	latestBlockHeight uint64
}

func New(dbPath string) *DAO {
	log.Debug("db path:", filepath.Join(dbPath, dbName))
	levelDB, err := leveldb.OpenFile(filepath.Join(dbPath, dbName), &opt.Options{})
	if err != nil {
		log.Fatalf("open leveldb [%s] error [%v]", dbName, err)
	}
	d := &DAO{DB: levelDB, latestBlockHash: nil, latestBlockHeight: 0}
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
	o.latestBlockHash = nil
	o.latestBlockHeight = 0
	return o.DoTransaction(func(trans *leveldb.Transaction) error {
		ro := &opt.ReadOptions{DontFillCache: false}
		value, err := o.DB.Get([]byte(latestBlockHashKey), ro)
		if err != nil {
			if err == leveldb.ErrNotFound {
				return errors.New("not found latest block hash")
			}
			return fmt.Errorf("load latest block hash failed: %v", err)
		}
		o.latestBlockHash = value

		value, err = o.DB.Get([]byte(latestBlockHeightKey), ro)
		if err != nil {
			if err == leveldb.ErrNotFound {
				return errors.New("not found latest block height")
			}
			return fmt.Errorf("load latest block height failed: %v", err)
		}
		o.latestBlockHeight, _ = strconv.ParseUint(string(value), 10, 64)

		return nil
	})
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

func (o *DAO) keyHeight(height uint64) []byte {
	key := heightPrefix + strconv.FormatUint(height, 16)
	return []byte(key)
}

func (o *DAO) GetBlockHash(height uint64) ([]byte, error) {
	return o.DB.Get(o.keyHeight(height), nil)
}

func (o *DAO) GetBlockByHeight(height uint64) ([]byte, error) {
	hash, err := o.GetBlockHash(height)
	if err != nil {
		return nil, fmt.Errorf("get block hash failed:%v", err)
	}
	return o.GetBlock(hash)
}

func (o *DAO) AddBlock(height uint64, hash [32]byte, block []byte) error {
	return o.DoTransaction(func(trans *leveldb.Transaction) error {
		wo := &opt.WriteOptions{Sync: true}

		err := trans.Put(hash[:], block, wo)
		if err != nil {
			return fmt.Errorf("intsert block failed: %v", err)
		}

		err = trans.Put(o.keyHeight(height), hash[:], wo)
		if err != nil {
			return fmt.Errorf("intsert block height-hash failed: %v", err)
		}

		err = trans.Put([]byte(latestBlockHashKey), hash[:], wo)
		if err != nil {
			return fmt.Errorf("update latest block hash failed: %v", err)
		}

		err = trans.Put([]byte(latestBlockHeightKey), []byte(strconv.FormatUint(height, 10)), wo)
		if err != nil {
			return fmt.Errorf("update latest block height failed: %v", err)
		}

		o.latestBlockHash = hash[:]
		o.latestBlockHeight = height
		return nil
	})
}

// RemoveLatestBlock 删除最后一个区块
func (o *DAO) RemoveLatestBlock() error {
	if o.latestBlockHash == nil {
		return nil
	}
	return o.DoTransaction(func(trans *leveldb.Transaction) error {
		err := trans.Delete(o.latestBlockHash, nil)
		if err != nil {
			return fmt.Errorf("delete latest block failed: %v", err)
		}

		err = trans.Delete(o.keyHeight(o.latestBlockHeight), nil)
		if err != nil {
			return fmt.Errorf("delete latest block hash failed: %v", err)
		}

		if o.latestBlockHeight == 0 {
			err = trans.Delete([]byte(latestBlockHashKey), nil)
			if err != nil {
				return err
			}

			err = trans.Delete([]byte(latestBlockHeightKey), nil)
			if err != nil {
				return err
			}

			o.latestBlockHash = nil
			o.latestBlockHeight = 0
			return nil
		}

		latestBlockHash, err := trans.Get(o.keyHeight(o.latestBlockHeight-1), nil)
		if err != nil {
			return fmt.Errorf("get latest block hash failed: %v", err)
		}

		err = trans.Put([]byte(latestBlockHashKey), latestBlockHash[:], nil)
		if err != nil {
			return fmt.Errorf("put latest block hash failed: %v", err)
		}

		err = trans.Put([]byte(latestBlockHeightKey), []byte(strconv.FormatUint(o.latestBlockHeight-1, 10)), nil)
		if err != nil {
			return fmt.Errorf("put latest block height failed: %v", err)
		}

		o.latestBlockHash = latestBlockHash
		o.latestBlockHeight--

		return nil
	})
}

// DoTransaction 事务操作
func (o *DAO) DoTransaction(fn func(trans *leveldb.Transaction) error) error {
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

	if err = fn(trans); err != nil {
		return err
	}

	err = trans.Commit()
	if err != nil {
		return fmt.Errorf("commit failed: %v", err)
	}

	return nil
}
