package config

import (
	"github.com/pkg/errors"
	pb "github.com/treeforest/easyblc/proto"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

type Config struct {
	// 对外服务配置
	HttpServerPort int `yaml:"http_server_port"` // web监听端口
	RpcServerPort  int `yaml:"rpc_server_port"`  // rpc服务监听端口

	// 区块链配置
	LevelDBPath string `yaml:"leveldb_path"` // 区块链数据库路径

	// 区块链节点配置
	Type              pb.NodeType `yaml:"type"`                // 节点类型
	Port              int         `yaml:"port"`                // 节点监听端口
	Endpoint          string      `yaml:"endpoint"`            // 节点对外暴露的地址
	BootstrapPeers    []string    `yaml:"bootstrap_peers"`     // 启动时连接的区块链节点地址
	BlockSyncInterval uint64      `yaml:"block_sync_interval"` // 同步间隔
	RewardAddress     string      `yaml:"reward_address"`      // 获取挖矿奖励的地址,若是矿工节点，则必须填写该选项

	// POW 配置
	//PowLimit                     uint32 // 挖矿最低难度值 0x1d00ffff
	//DifficultyAdjustmentInterval uint64 // 难度调整的区块间隔 2016
	//PowTargetTimespan            int64  // 规定的出块时间，DifficultyAdjustmentInterval个区块的出块时间 2016*60*10
}

func DefaultConfig() *Config {
	return &Config{
		HttpServerPort:    8080,
		RpcServerPort:     8081,
		LevelDBPath:       ".",
		Type:              pb.NodeType_Full,
		Port:              4499,
		Endpoint:          "localhost:4499",
		BootstrapPeers:    []string{},
		BlockSyncInterval: 1,
		RewardAddress:     "",
	}
}

func (c *Config) Unmarshal(b []byte) error {
	return yaml.Unmarshal(b, c)
}

func (c *Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

func Load(path string) (*Config, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	conf := new(Config)
	if err = conf.Unmarshal(data); err != nil {
		return nil, errors.WithStack(err)
	}

	return conf, nil
}
