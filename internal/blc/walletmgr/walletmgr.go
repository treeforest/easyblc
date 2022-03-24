package walletmgr

import (
	"errors"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc/walletmgr/wallet"
	"github.com/treeforest/easyblc/pkg/gob"
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
	if err = gob.DecodeP256(data, m); err != nil {
		log.Fatal("decode p256 failed: ", err)
	}
}

func (m *WalletManager) CreateWallet() error {
	w := wallet.New()
	address := string(w.GetAddress())
	m.Keys[address] = len(m.Wallets)
	m.Wallets = append(m.Wallets, w)
	return m.save()
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

func (m *WalletManager) Has(address string) bool {
	for _, addr := range m.Addresses() {
		if addr == address {
			return true
		}
	}
	return false
}

func (m *WalletManager) save() error {
	data, err := gob.EncodeP256(m)
	if err != nil {
		return fmt.Errorf("encode p256 failed: %v", err)
	}
	err = ioutil.WriteFile(walletFilename, data, 07644)
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
