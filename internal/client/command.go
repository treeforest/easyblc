package client

import (
	"flag"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc"
	"github.com/treeforest/easyblc/internal/blc/walletmgr"
	"github.com/treeforest/easyblc/pkg/utils"
	log "github.com/treeforest/logger"
	"os"
)

type Command struct{}

func New() *Command {
	return &Command{}
}

func (c *Command) printUsage() {
	fmt.Println("Usage:")
	// 初始化区块链
	fmt.Printf("\tcreateblockchain -address ADDRESS -- 创建区块链\n")
	fmt.Printf("\t\t-address -- 接收创世区块奖励的地址\n")
	// 打印区块信息
	fmt.Printf("\tprinfchain -- 输出区块链信息\n")
	// 转账 todo
	fmt.Printf("\tsend -from FROM -to TO -amount AMOUNT -- 发起转账\n")
	fmt.Printf("\t\t-from FROM -- 转账源地址\n")
	fmt.Printf("\t\t-to TO -- 转账目标地址\n")
	fmt.Printf("\t\t-AMOUNT amount -- 转账金额\n")
	// 钱包
	fmt.Printf("\tcreatewallet -- 创建钱包\n")
	fmt.Printf("\tremovewallet -- 删除钱包\n")
	fmt.Printf("\t\t-address ACCOUNT -- 钱包地址\n")
	fmt.Printf("\taddresses -- 获取钱包地址列表\n")
	// 余额
	fmt.Printf("\tgetbalance -- 获取钱包余额\n")
	fmt.Printf("\t\t-address ACCOUNT -- 钱包地址\n")
}

func (c *Command) Run() {
	// 创世交易
	cmdCreateBlockchain := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	address := cmdCreateBlockchain.String("address", "test", "接收创世区块奖励的地址")
	// 输出区块链信息
	cmdPrintChain := flag.NewFlagSet("printchain", flag.ExitOnError)
	// 转账
	cmdSend := flag.NewFlagSet("send", flag.ExitOnError)
	from := cmdSend.String("from", "", "转账源地址")
	to := cmdSend.String("to", "", "转账目标地址")
	amount := cmdSend.Uint("amount", 0, "转账金额")
	// 钱包
	cmdCreateWallet := flag.NewFlagSet("createwallet", flag.ExitOnError)
	cmdRemoveWallet := flag.NewFlagSet("removewallet", flag.ExitOnError)
	argRemoveWalletAddress := cmdRemoveWallet.String("address", "", "钱包地址")
	cmdAddresses := flag.NewFlagSet("addresses", flag.ExitOnError)
	// 余额
	cmdGetBalance := flag.NewFlagSet("getbalance", flag.ExitOnError)

	if len(os.Args) < 2 {
		c.printUsage()
		return
	}

	switch os.Args[1] {
	case "createblockchain":
		if !c.parseCommand(cmdCreateBlockchain) || *address == "" {
			goto HELP
		}
		c.createBlockChainWithGenesisBlock(*address)
	case "printchain":
		if !c.parseCommand(cmdPrintChain) {
			goto HELP
		}
		c.printChain()
	case "send":
		if !c.parseCommand(cmdSend) || *from == "" || *to == "" || *amount <= 0 {
			goto HELP
		}
		c.send(*from, *to, uint32(*amount))
	case "createwallet":
		if !c.parseCommand(cmdCreateWallet) {
			goto HELP
		}
		c.createWallet()
	case "removewallet":
		if !c.parseCommand(cmdRemoveWallet) || *argRemoveWalletAddress == "" {
			goto HELP
		}
		c.removeWallet(*argRemoveWalletAddress)
	case "addresses":
		if !c.parseCommand(cmdAddresses) {
			goto HELP
		}
		c.printAddresses()
	case "getbalance":
		if !c.parseCommand(cmdGetBalance) {
			goto HELP
		}
		c.getBalance()
	default:
		goto HELP
	}
	return
HELP:
	c.printUsage()
}

func (c *Command) getBalance() {
	bc := blc.GetBlockChain()
	defer bc.Close()

	total := uint32(0)
	res := ""
	mgr := walletmgr.New()
	for _, addr := range mgr.Addresses() {
		amount, err := bc.GetBalance(addr)
		if err != nil {
			log.Fatal("get balance failed:", err)
		}
		total += amount
		res = fmt.Sprintf("%s\n\t地址: %s\t余额: %d", res, addr, amount)
	}
	res = fmt.Sprintf("总余额:%d%s", total, res)

	fmt.Println(res)
}

func (c *Command) send(from, to string, amount uint32) {
	if utils.IsValidAddress(from) {
		log.Fatal("FROM is not a valid address")
	}
	if utils.IsValidAddress(to) {
		log.Fatal("TO is not a valid address")
	}
	mgr := walletmgr.New()
	if !mgr.Has(from) {
		log.Fatal("you don't have the wallet that address is ", from)
	}

	bc := blc.GetBlockChain()
	defer bc.Close()
	balance, err := bc.GetBalance(from)
	if err != nil {
		log.Fatal("get balance failed: ", err)
	}
	if balance < amount {
		log.Fatal("余额不足")
	}

	// 创建交易

	// 挖矿
}

func (c *Command) parseCommand(f *flag.FlagSet) bool {
	if err := f.Parse(os.Args[2:]); err != nil {
		log.Fatal("parse command failed!")
	}
	if f.Parsed() {
		return true
	}
	return false
}

func (c *Command) printAddresses() {
	mgr := walletmgr.New()
	fmt.Println("地址列表：")
	for _, address := range mgr.Addresses() {
		fmt.Printf("\t%s\n", address)
	}
}

func (c *Command) removeWallet(address string) {
	mgr := walletmgr.New()
	err := mgr.RemoveWallet(address)
	if err != nil {
		log.Warn(err)
		return
	}
	log.Info("remove wallet success")
}

func (c *Command) createWallet() {
	mgr := walletmgr.New()
	if err := mgr.CreateWallet(); err != nil {
		log.Fatal("create wallet failed: ", err)
	}
	log.Info("create wallet success")
}

func (c *Command) createBlockChainWithGenesisBlock(address string) {
	bc := blc.CreateBlockChainWithGenesisBlock(address)
	defer bc.Close()
	log.Info("create block chain success")
}

func (c *Command) printChain() {
	bc := blc.GetBlockChain()
	defer bc.Close()

	fmt.Println("blockchain:")
	err := bc.Traverse(func(block *blc.Block) {
		fmt.Printf("\tHeight:%d\n", block.Height)
		fmt.Printf("\tHash:%x\n", block.Hash)
		fmt.Printf("\tPrevkHash:%x\n", block.PreHash)
		fmt.Printf("\tTime:%v\n", block.Time)
		fmt.Printf("\tNonce:%d\n", block.Nonce)
		fmt.Printf("\tMerkelRoot:%x\n", block.MerkelRoot)
		fmt.Printf("\tBits:%d\n", block.Bits)
		fmt.Printf("\tTransactions:\n")
		for _, tx := range block.Transactions {
			fmt.Printf("\t\thash:%x\n", tx.Hash) // 交易哈希
			fmt.Printf("\t\tVins:\n")
			for _, vin := range tx.Vins {
				fmt.Printf("\t\t\ttxid: %x\n", vin.TxId)
				fmt.Printf("\t\t\tvout: %d\n", vin.Vout)
				if vin.IsCoinbase() {
					fmt.Printf("\t\t\tcoinbaseDataSize: %d\n", vin.CoinbaseDataSize)
					fmt.Printf("\t\t\tcoinbaseData: %s\n", vin.CoinbaseData)
				} else {
					fmt.Printf("\t\t\tscriptSig: %x\n", vin.ScriptSig)
				}
			}
			fmt.Printf("\t\tVouts:\n")
			for _, vout := range tx.Vouts {
				fmt.Printf("\t\t\tvalue: %d\n", vout.Value)
				fmt.Printf("\t\t\taddress: %s\n", vout.Address)
				fmt.Printf("\t\t\tscriptPubKey: %x\n", vout.ScriptPubKey)
			}
		}
	})
	if err != nil {
		log.Fatalf("traverse block error:%v", err)
	}
}
