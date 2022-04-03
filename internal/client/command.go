package client

import (
	"context"
	"flag"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc"
	"github.com/treeforest/easyblc/internal/blc/walletmgr"
	"github.com/treeforest/easyblc/pkg/utils"
	log "github.com/treeforest/logger"
	"math/rand"
	"os"
	"time"
)

type Command struct {
	dbPath string
}

func New(dbPath string) *Command {
	return &Command{dbPath}
}

func (c *Command) printUsage() {
	fmt.Println("Usage:")
	// 初始化区块链
	fmt.Printf("\tcreateblockchain -address ADDRESS -- 创建区块链\n")
	fmt.Printf("\t\t-address -- 接收创世区块奖励的地址\n")
	// 打印区块信息
	fmt.Printf("\tprintchain -- 输出区块链信息\n")
	// 转账
	fmt.Printf("\tsend -from FROM -to TO -amount AMOUNT -- 发起转账\n")
	fmt.Printf("\t\t-from FROM -- 转账源地址\n")
	fmt.Printf("\t\t-to TO -- 转账目标地址\n")
	fmt.Printf("\t\t-amount AMOUNT -- 转账金额\n")
	// 挖矿
	fmt.Printf("\tmining -address ADDRESS -t T -- 开始挖矿\n")
	fmt.Printf("\t\t-address -- 接收挖矿奖励地址\n")
	fmt.Printf("\t\t-t -- 挖矿个数\n")
	// 钱包
	fmt.Printf("\tcreatewallet -- 创建钱包\n")
	fmt.Printf("\tremovewallet -- 删除钱包\n")
	fmt.Printf("\t\t-address ACCOUNT -- 钱包地址\n")
	fmt.Printf("\taddresses -- 获取钱包地址列表\n")
	// 余额
	fmt.Printf("\tgetbalance -- 获取钱包余额\n")
	// 交易池
	fmt.Printf("\ttxpool -- 输出交易池")
	// 高度
	fmt.Printf("\theight -- 打印区块链高度")
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
	// 挖矿
	cmdMining := flag.NewFlagSet("mining", flag.ExitOnError)
	argMiningRewardAddress := cmdMining.String("address", "", "接收挖矿奖励地址")
	argMiningNum := cmdMining.Int("n", 1, "挖矿个数")
	// 钱包
	cmdCreateWallet := flag.NewFlagSet("createwallet", flag.ExitOnError)
	cmdRemoveWallet := flag.NewFlagSet("removewallet", flag.ExitOnError)
	argRemoveWalletAddress := cmdRemoveWallet.String("address", "", "钱包地址")
	cmdAddresses := flag.NewFlagSet("addresses", flag.ExitOnError)
	// 余额
	cmdGetBalance := flag.NewFlagSet("getbalance", flag.ExitOnError)
	// 交易池
	cmdTxPool := flag.NewFlagSet("txpool", flag.ExitOnError)
	// 区块高度
	cmdHeight := flag.NewFlagSet("height", flag.ExitOnError)

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
		c.send(*from, *to, uint64(*amount))
	case "mining":
		if !c.parseCommand(cmdMining) || *argMiningRewardAddress == "" || *argMiningNum < 1 {
			goto HELP
		}
		c.mining(*argMiningRewardAddress, *argMiningNum)
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
	case "txpool":
		if !c.parseCommand(cmdTxPool) {
			goto HELP
		}
		c.printTxPool()
	case "height":
		if !c.parseCommand(cmdHeight) {
			goto HELP
		}
		c.printHeight()
	default:
		goto HELP
	}
	return
HELP:
	c.printUsage()
}

func (c *Command) printHeight() {
	bc := blc.MustGetExistBlockChain(c.dbPath)
	defer bc.Close()
	log.Infof("当前高度：%d", bc.GetLatestBlock().Height)
}

func (c *Command) printTxPool() {
	bc := blc.MustGetExistBlockChain(c.dbPath)
	defer bc.Close()

	fmt.Println("tx pool: ")
	bc.GetTxPool().Traverse(func(fee uint64, tx *blc.Transaction) {
		fmt.Printf("\tfee: %d\n", fee)
		fmt.Printf("\ttxHash: %x\n", tx.Hash)
		fmt.Printf("\ttime: %d\n", tx.Time)
		fmt.Printf("\tVins:\n")
		for _, vin := range tx.Vins {
			fmt.Printf("\t\ttxid: %x\n", vin.TxId)
			fmt.Printf("\t\tvout: %d\n", vin.Vout)
			if vin.IsCoinbase() {
				fmt.Printf("\t\tcoinbaseDataSize: %d\n", vin.CoinbaseDataSize)
				fmt.Printf("\t\tcoinbaseData: %s\n", vin.CoinbaseData)
			} else {
				fmt.Printf("\t\tscriptSig: %x\n", vin.ScriptSig)
			}
		}
		fmt.Printf("\tVouts:\n")
		for index, vout := range tx.Vouts {
			fmt.Printf("\t\tindex: %d\n", index)
			fmt.Printf("\t\tvalue: %d\n", vout.Value)
			fmt.Printf("\t\taddress: %s\n", vout.Address)
			fmt.Printf("\t\tscriptPubKey: %x\n", vout.ScriptPubKey)
		}
		fmt.Printf("\n")
	})
}

func (c *Command) getBalance() {
	bc := blc.MustGetExistBlockChain(c.dbPath)
	defer bc.Close()

	total := uint64(0)
	res := ""
	mgr := walletmgr.New()
	for _, addr := range mgr.Addresses() {
		amount := bc.GetBalance(addr)
		total += amount
		res = fmt.Sprintf("%s\n\t地址: %s\t余额: %d", res, addr, amount)
	}
	res = fmt.Sprintf("总余额:%d%s", total, res)

	fmt.Println(res)
}

func (c *Command) mining(address string, n int) {
	// 检查地址格式
	if !utils.IsValidAddress(address) {
		log.Fatal("ADDRESS is not a valid address")
	}
	bc := blc.MustGetExistBlockChain(c.dbPath)
	defer bc.Close()

	for i := 0; i < n; i++ {
		if err := bc.MineBlock(context.Background(), address); err != nil {
			log.Fatal("mine block failed:", err)
		}
	}
}

func (c *Command) send(from, to string, amount uint64) {
	// 检查地址格式
	if !utils.IsValidAddress(from) {
		log.Fatal("FROM is not a valid address")
	}
	if !utils.IsValidAddress(to) {
		log.Fatal("TO is not a valid address")
	}

	// 检查from是否是钱包地址
	mgr := walletmgr.New()
	if !mgr.Has(from) {
		log.Fatal("you don't have the wallet that address is ", from)
	}
	wallet, err := mgr.GetWallet(from)
	if err != nil {
		log.Fatal("not found wallet:", err)
	}

	bc := blc.MustGetExistBlockChain(c.dbPath)
	defer bc.Close()

	// 检查余额
	log.Debug("check balance...")
	balance := bc.GetBalance(from)
	if balance < amount {
		log.Fatal("lack of balance")
	}

	// 交易输入
	log.Debug("check utxo...")
	var vins []*blc.TxInput
	utxoSet := bc.GetUTXOSet(from)
	utxoSet.Traverse(func(txHash [32]byte, index int, output *blc.TxOutput) {
		vin, err := blc.NewTxInput(txHash, uint32(index), &wallet.Key)
		if err != nil {
			log.Fatal("create tx vin failed:", err)
		}
		vins = append(vins, vin)
	})

	// 生成一个找零地址
	addresses := mgr.Addresses()
	rand.Seed(time.Now().UnixNano())
	changeAddress := addresses[rand.Intn(len(addresses))]
	log.Debug("随机找零地址：", changeAddress)

	// 手续费
	fee := uint64(0)
	if balance-amount > 50 {
		fee = 50
	}
	log.Debug("找零:", balance-amount-fee)
	log.Debug("手续费:", fee)

	// 交易输出
	log.Debug("create transaction output...")
	var vouts []*blc.TxOutput
	vout, err := blc.NewTxOutput(amount, to)
	if err != nil {
		log.Fatal("create tx vout failed:", err)
	}
	changeVout, err := blc.NewTxOutput(balance-amount-fee, changeAddress)
	if err != nil {
		log.Fatal("create tx vout failed:", err)
	}
	vouts = append(vouts, []*blc.TxOutput{vout, changeVout}...)

	// 构造交易
	log.Debug("create transaction...")
	tx, err := blc.NewTransaction(vins, vouts)
	if err != nil {
		log.Fatal("create transaction failed:", err)
	}

	// 将交易放入交易池
	log.Debug("put transaction to txpool...")
	if err = bc.AddToTxPool(tx); err != nil {
		log.Fatal("put tx to pool failed:", err)
	}

	// 挖矿
	//if err = bc.MineBlock(from); err != nil {
	//	log.Fatal("mine block failed:", err)
	//}
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
	w, err := mgr.CreateWallet()
	if err != nil {
		log.Fatal("create wallet failed: ", err)
	}
	log.Infof("create wallet success, address=%s", w.GetAddress())
}

func (c *Command) createBlockChainWithGenesisBlock(address string) {
	bc := blc.CreateBlockChainWithGenesisBlock(c.dbPath, address)
	defer bc.Close()
	log.Info("create block chain success")
}

func (c *Command) printChain() {
	bc := blc.MustGetExistBlockChain(c.dbPath)
	defer bc.Close()

	fmt.Println("blockchain:")
	err := bc.Traverse(func(block *blc.Block) bool {
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
			for index, vout := range tx.Vouts {
				fmt.Printf("\t\t\tindex: %d\n", index)
				fmt.Printf("\t\t\tvalue: %d\n", vout.Value)
				fmt.Printf("\t\t\taddress: %s\n", vout.Address)
				fmt.Printf("\t\t\tscriptPubKey: %x\n", vout.ScriptPubKey)
			}
			fmt.Printf("\n")
		}
		return true
	})
	if err != nil {
		log.Fatalf("traverse block error:%v", err)
	}
}
