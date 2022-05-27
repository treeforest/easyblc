package blc

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/treeforest/easyblc/walletmgr/wallet"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	pb "github.com/treeforest/easyblc/proto"
	"github.com/treeforest/easyblc/walletmgr"
	log "github.com/treeforest/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type rpcClient struct {
	address string
}

func NewRpcClient(address string) *rpcClient {
	return &rpcClient{address: address}
}

func (c *rpcClient) printUsage() {
	fmt.Println("Usage:")
	// 高度
	fmt.Printf("\theight -- 打印区块链高度\n")
	// 打印区块信息
	fmt.Printf("\tprintchain -- 输出区块链信息\n")
	// 转账
	fmt.Printf("\tsend -from FROM -to TO -amount AMOUNT -- 发起转账\n")
	fmt.Printf("\t\t-from FROM -- 转账源地址,多个时采用,分割\n")
	fmt.Printf("\t\t-to TO -- 转账目标地址,多个时采用,分割\n")
	fmt.Printf("\t\t-amount AMOUNT -- 转账金额,多个时采用,分割\n")
	fmt.Printf("\t\t-fee FEE -- 支付的手续费\n")
	// 钱包
	fmt.Printf("\tcreatewallet -- 创建钱包\n")
	fmt.Printf("\tremovewallet -address ADDRESS -- 删除钱包\n")
	fmt.Printf("\t\t-address ADDRESS -- 钱包地址\n")
	fmt.Printf("\taddresses -- 获取钱包地址列表\n")
	// 余额
	fmt.Printf("\tallbalance -- 获取所有的余额")
	fmt.Printf("\tbalance -address ADDRESS -- 获取钱包余额\n")
	fmt.Printf("\t\t-address ADDRESS -- 钱包地址\n")
	// 交易池
	fmt.Printf("\ttxpool -- 获取交易池内的交易信息")
}

func (c *rpcClient) Run() {
	// 区块高度
	cmdHeight := flag.NewFlagSet("height", flag.ExitOnError)
	// 输出区块链信息
	cmdPrintChain := flag.NewFlagSet("printchain", flag.ExitOnError)
	// 转账
	cmdSend := flag.NewFlagSet("send", flag.ExitOnError)
	from := cmdSend.String("from", "", "转账源地址,多个时采用,分割")
	to := cmdSend.String("to", "", "转账目标地址,多个时采用,分割")
	amount := cmdSend.String("amount", "", "转账金额,多个时采用,分割")
	fee := cmdSend.Uint64("fee", 0, "支付的手续费")
	// 钱包
	cmdCreateWallet := flag.NewFlagSet("createwallet", flag.ExitOnError)
	cmdRemoveWallet := flag.NewFlagSet("removewallet", flag.ExitOnError)
	argRemoveWalletAddress := cmdRemoveWallet.String("address", "", "钱包地址")
	cmdAddresses := flag.NewFlagSet("addresses", flag.ExitOnError)
	// 余额
	cmdAllBalance := flag.NewFlagSet("allbalance", flag.ExitOnError)
	cmdBalance := flag.NewFlagSet("balance", flag.ExitOnError)
	argBalanceAddress := cmdBalance.String("address", "", "钱包地址")
	// 交易池
	cmdTxPool := flag.NewFlagSet("txpool", flag.ExitOnError)

	if len(os.Args) < 2 {
		c.printUsage()
		return
	}

	switch os.Args[1] {
	case "height":
		if !parseCommand(cmdHeight) {
			goto HELP
		}
		c.printHeight()
	case "printchain":
		if !parseCommand(cmdPrintChain) {
			goto HELP
		}
		c.printChain()
	case "send":
		if !parseCommand(cmdSend) || *from == "" || *to == "" || *amount == "" {
			goto HELP
		}
		c.send(*from, *to, *amount, *fee)
	case "createwallet":
		if !parseCommand(cmdCreateWallet) {
			goto HELP
		}
		c.createWallet()
	case "removewallet":
		if !parseCommand(cmdRemoveWallet) || *argRemoveWalletAddress == "" {
			goto HELP
		}
		c.removeWallet(*argRemoveWalletAddress)
	case "addresses":
		if !parseCommand(cmdAddresses) {
			goto HELP
		}
		c.printAddresses()
	case "balance":
		if !parseCommand(cmdBalance) || *argBalanceAddress == "" {
			goto HELP
		}
		c.printBalance(*argBalanceAddress)
	case "allbalance":
		if !parseCommand(cmdAllBalance) {
			goto HELP
		}
		c.printAllBalance()
	case "txpool":
		if !parseCommand(cmdTxPool) {
			goto HELP
		}
		c.printTxPool()
	default:
		goto HELP
	}
	return
HELP:
	c.printUsage()
}

func (c *rpcClient) send(from, to, amount string, fee uint64) {
	inputAddrs := strings.Split(from, ",")
	outputAddrs := strings.Split(to, ",")
	amounts := strings.Split(amount, ",")

	if len(outputAddrs) != len(amounts) {
		log.Fatal("输出地址与输出金额没有匹配到")
	}

	for _, addr := range outputAddrs {
		if !IsValidAddress(addr) {
			log.Fatalf("%s is not a valid address", addr)
		}
	}

	inputAmount := uint64(0)
	outputAmount := uint64(0)

	mgr := walletmgr.New()
	for _, addr := range inputAddrs {
		if !mgr.Has(addr) {
			log.Fatalf("you don't have the address %s in  wallet", addr)
		}
		if !IsValidAddress(addr) {
			log.Fatalf("%s is not a valid address", addr)
		}
		balance := c.getBalance(addr)
		if balance == 0 {
			log.Fatalf("address %s balance is 0", addr)
		}
		inputAmount += balance
	}

	uint64Amounts := make([]uint64, 0)
	for _, a := range amounts {
		n, err := strconv.ParseUint(a, 10, 64)
		if err != nil {
			log.Fatalf("%s can't conv uint64:%v", a, err)
		}
		uint64Amounts = append(uint64Amounts, n)
		outputAmount += n
	}

	if inputAmount < (outputAmount + fee) {
		log.Fatal("输入金额小于输出金额")
	}

	// 1.交易输出
	// 1.1 构造交易输出
	vouts := make([]TxOutput, 0)
	for i, addr := range outputAddrs {
		vout, err := NewTxOutput(uint64Amounts[i], addr)
		if err != nil {
			log.Fatal("create tx vout failed:", err)
		}
		vouts = append(vouts, *vout)
	}
	// 1.2 输入金额大于输出金额的部分就是找零金额
	if inputAmount > (outputAmount + fee) {
		log.Info("找零: ", inputAmount-(outputAmount+fee))
		// 生成随机找零地址
		addrs := mgr.Addresses()
		rand.Seed(time.Now().UnixNano())
		addr := addrs[rand.Intn(len(addrs))] // 找零地址
		vout, err := NewTxOutput(inputAmount-(outputAmount+fee), addr)
		if err != nil {
			log.Fatal("create tx vout failed:", err)
		}
		vouts = append(vouts, *vout)
	}

	// 2. 交易输入
	// 2.1 构造交易输入
	vins := make([]TxInput, 0)
	ws := make([]*wallet.Wallet, 0)
	for _, addr := range inputAddrs {
		utxo := c.getUtxo(addr)
		if utxo == nil {
			log.Fatalf("no utxo with address %s", addr)
		}
		w, err := mgr.GetWallet(addr)
		if err != nil {
			log.Fatal("not found wallet:", err)
		}
		ws = append(ws, w) // 记录待会要进行签名的钱包
		for txHash, outputs := range utxo {
			for index, _ := range outputs {
				vin, err := NewTxInput(txHash, uint32(index))
				if err != nil {
					log.Fatal("create tx vin failed:", err)
				}
				vins = append(vins, *vin)
			}
		}
	}

	// 3. 生成交易
	tx, err := NewTransaction(vins, vouts)
	if err != nil {
		log.Fatal("create transaction failed:", err)
	}

	// 4. 设置交易输入脚本
	for i := 0; i < len(tx.Vins); i++ {
		if err = tx.Vins[i].SetScriptSig(tx.Hash, &ws[i].Key); err != nil {
			log.Fatal(err)
		}
	}

	// 5. 将交易放入交易池
	c.postTx(tx)
}

func (c *rpcClient) printAllBalance() {
	mgr := walletmgr.New()
	info := map[string]uint64{}
	total := uint64(0)
	for _, address := range mgr.Addresses() {
		balance := c.getBalance(address)
		total += balance
		info[address] = balance
	}
	fmt.Println("总余额: ", total)
	for _, address := range mgr.Addresses() {
		fmt.Printf("\t地址：%s\t余额:%d\n", address, info[address])
	}
}

func (c *rpcClient) printBalance(address string) {
	log.Info("balance: ", c.getBalance(address))
}

func (c *rpcClient) printTxPool() {
	var poolData []byte
	c.call(func(client pb.ApiClient) {
		resp, err := client.GetTxPool(context.Background(), &pb.GetTxPoolReq{})
		if err != nil {
			log.Fatal("rpc error: ", err)
		}
		poolData = resp.TxPool
	})

	pool := TxPool{}
	if err := json.Unmarshal(poolData, &pool); err != nil {
		log.Fatal(err)
	}

	pool.Traverse(func(fee uint64, tx *Transaction) bool {
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
		return true
	})
}

func (c *rpcClient) printHeight() {
	log.Info("height: ", c.getHeight())
}

func (c *rpcClient) printChain() {
	height := c.getHeight()
	log.Info("blockchain height: ", height)
	blocks := c.getBlocks(0, height)
	for _, block := range blocks {
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
	}
}

func (c *rpcClient) printAddresses() {
	mgr := walletmgr.New()
	fmt.Println("地址列表：")
	for _, address := range mgr.Addresses() {
		fmt.Printf("\t%s\n", address)
	}
}

func (c *rpcClient) removeWallet(address string) {
	mgr := walletmgr.New()
	err := mgr.RemoveWallet(address)
	if err != nil {
		log.Warn(err)
		return
	}
	log.Info("remove wallet success")
}

func (c *rpcClient) createWallet() {
	mgr := walletmgr.New()
	w, err := mgr.CreateWallet()
	if err != nil {
		log.Fatal("create wallet failed: ", err)
	}
	log.Infof("create wallet success, address=%s", w.GetAddress())
}

func (c *rpcClient) postTx(tx *Transaction) {
	c.call(func(client pb.ApiClient) {
		ins := make([]pb.TxInput, 0)
		for _, in := range tx.Vins {
			ins = append(ins, pb.TxInput{
				TxId:             in.TxId[:],
				Vout:             in.Vout,
				ScriptSig:        in.ScriptSig,
				CoinbaseDataSize: int32(in.CoinbaseDataSize),
				CoinbaseData:     in.CoinbaseData,
			})
		}
		outs := make([]pb.TxOutput, 0)
		for _, out := range tx.Vouts {
			outs = append(outs, pb.TxOutput{
				Value:        out.Value,
				Address:      out.Address,
				ScriptPubKey: out.ScriptPubKey,
			})
		}
		req := &pb.PostTxReq{
			Tx: pb.Transaction{
				Hash:      tx.Hash[:],
				Ins:       ins,
				Outs:      outs,
				Timestamp: tx.Time,
			},
		}
		_, err := client.PostTx(context.Background(), req)
		if err != nil {
			log.Fatal("交易提交失败:", err)
		}
		log.Info("交易提交成功")
	})
}

func (c *rpcClient) getUtxo(address string) map[[32]byte]map[int]*TxOutput {
	var utxos []pb.UTXO
	c.call(func(client pb.ApiClient) {
		resp, err := client.GetUTXO(context.Background(), &pb.GetUTXOReq{Address: address})
		if err != nil {
			log.Fatal("rpc error ", err)
		}
		utxos = resp.Utxos
	})

	if len(utxos) == 0 {
		return nil
	}

	mp := map[[32]byte]map[int]*TxOutput{}
	for _, utxo := range utxos {
		hash, err := BytesToByte32(utxo.TxHash)
		if err != nil {
			log.Fatal(err)
		}
		if _, ok := mp[hash]; !ok {
			mp[hash] = map[int]*TxOutput{}
		}
		mp[hash][int(utxo.Index)] = &TxOutput{
			Value:        utxo.TxOut.Value,
			Address:      utxo.TxOut.Address,
			ScriptPubKey: utxo.TxOut.ScriptPubKey,
		}
	}

	return mp
}

func (c *rpcClient) getBalance(address string) uint64 {
	var balance uint64
	c.call(func(client pb.ApiClient) {
		resp, err := client.GetBalance(context.Background(), &pb.GetBalanceReq{Address: address})
		if err != nil {
			log.Fatal("rpc error: ", err)
		}
		balance = resp.Balance
	})
	return balance
}

func (c *rpcClient) getBlocks(start, end uint64) []Block {
	var data []byte
	c.call(func(client pb.ApiClient) {
		resp, err := client.GetBlocks(context.Background(), &pb.GetBlocksReq{
			StartHeight: start,
			EndHeight:   end,
		})
		if err != nil {
			log.Fatal("rpc error ", err)
		}
		data = resp.Blocks
	})

	blocks := make([]Block, 0)
	if err := json.Unmarshal(data, &blocks); err != nil {
		log.Fatal("json unmarshal ", err)
	}

	return blocks
}

func (c *rpcClient) getHeight() uint64 {
	var height uint64
	c.call(func(client pb.ApiClient) {
		resp, err := client.GetHeight(context.Background(), &pb.GetHeightReq{})
		if err != nil {
			log.Fatal("rpc error ", err)
		}
		height = resp.Height
	})
	return height
}

func (c *rpcClient) call(f func(client pb.ApiClient)) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	dialOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	}
	cc, err := grpc.DialContext(ctx, c.address, dialOpts...)
	if err != nil {
		log.Fatalf("dial error: %+v", err)
	}
	defer cc.Close()
	f(pb.NewApiClient(cc))
}

func parseCommand(f *flag.FlagSet) bool {
	if err := f.Parse(os.Args[2:]); err != nil {
		log.Fatal("parse command failed!")
	}
	if f.Parsed() {
		return true
	}
	return false
}
