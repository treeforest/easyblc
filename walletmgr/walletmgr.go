package walletmgr

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/treeforest/easyblc/walletmgr/wallet"
	"io/ioutil"
	"log"
	"os"
)

const (
	walletFilename = "wallets.bat"
)

// WalletManager 钱包管理器
type WalletManager struct {
	Keys    map[string]int   // address => index 快速删除
	Wallets []*wallet.Wallet // 链表确保有序
	//Wallets map[string]*wallet.Wallet // address => wallet
}

func New() *WalletManager {
	mgr := WalletManager{Keys: map[string]int{}, Wallets: []*wallet.Wallet{}}
	mgr.init()
	return &mgr
}

func (m *WalletManager) init() {
	if !m.isExistWalletFile() {
		return
	}
	data, err := ioutil.ReadFile(walletFilename)
	if err != nil {
		log.Fatal("read wallet file failed:", data)
	}

	gob.Register(elliptic.P256())
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err = dec.Decode(m); err != nil {
		log.Fatal("decode p256 failed: ", err)
	}
}

func (m *WalletManager) CreateWallet() (*wallet.Wallet, error) {
	w := wallet.New()
	address := string(w.GetAddress())
	m.Keys[address] = len(m.Wallets)
	m.Wallets = append(m.Wallets, w)
	if err := m.save(); err != nil {
		return nil, err
	}
	return w, nil
}

func (m *WalletManager) RemoveWallet(address string) error {
	i, ok := m.Keys[address]
	if !ok {
		return errors.New("address not exist")
	}
	m.Wallets = append(m.Wallets[:i], m.Wallets[i+1:]...)
	delete(m.Keys, address)
	return m.save()
}

func (m *WalletManager) Addresses() []string {
	var addresses []string
	for _, w := range m.Wallets {
		addresses = append(addresses, string(w.GetAddress()))
	}
	return addresses
}

func (m *WalletManager) GetWallet(address string) (*wallet.Wallet, error) {
	i, has := m.Keys[address]
	if !has {
		return nil, errors.New("not found address")
	}
	return m.Wallets[i], nil
}

func (m *WalletManager) Has(address string) bool {
	_, has := m.Keys[address]
	return has
}

func (m *WalletManager) save() error {
	buf := bytes.NewBuffer(nil)
	gob.Register(elliptic.P256())
	enc := gob.NewEncoder(buf)
	if err := enc.Encode(m); err != nil {
		return fmt.Errorf("encode failed: %v", err)
	}
	err := ioutil.WriteFile(walletFilename, buf.Bytes(), 07644)
	if err != nil {
		return fmt.Errorf("write wallet file failed:%v", err)
	}
	return nil
}

func (m *WalletManager) isExistWalletFile() bool {
	if _, err := os.Stat(walletFilename); os.IsNotExist(err) {
		return false
	}
	return true
}
