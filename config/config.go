package config

import (
	"flag"
	"fmt"
	log "github.com/treeforest/logger"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

var (
	path *string
)

func init() {
	path = flag.String("conf", "config.yaml", "p2p server config path")
}

type Config struct {
	Port           int      `yaml:"port"`           // gRPC监听端口
	Endpoint       string   `yaml:"endpoint"`       // 节点对外暴露的gRPC地址
	Type           uint32   `yaml:"type"`           // 节点类型
	DBPath         string   `yaml:"dbpath"`         // 区块链数据库路径
	BootstrapPeers []string `yaml:"bootstrapPeers"` // 启动时连接的区块链节点地址
	RewardAddress  string   `yaml:"rewardAddress"`  // 获取挖矿奖励的地址
	SyncInterval   uint64   `yaml:"syncInterval"`   // 同步间隔
	HttpServerPort int      `yaml:"httpServerPort"` // web监听端口
}

func Load() (*Config, error) {
	flag.Parse()

	data, err := ioutil.ReadFile(*path)
	if err != nil {
		return nil, fmt.Errorf("read config file failed: %v", err)
	}

	out := new(Config)
	err = yaml.Unmarshal(data, out)
	if err != nil {
		return nil, fmt.Errorf("unmarshal config failed: %v", err)
	}

	data, _ = yaml.Marshal(out)
	log.Info("config:\n", string(data))

	return out, nil
}
