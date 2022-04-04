package client

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/treeforest/easyblc/internal/blc"
	"github.com/treeforest/easyblc/internal/blc/walletmgr"
	"github.com/treeforest/easyblc/pkg/utils"
	log "github.com/treeforest/logger"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	BaseUrl string // 基础url
}

func LoadConfig() *Config {
	data, err := ioutil.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("read file failed:", err)
	}
	c := &Config{}
	err = yaml.Unmarshal(data, c)
	if err != nil {
		log.Fatal("unmarshal failed:", err)
	}
	return c
}

type HttpClient struct {
	baseUrl string
}

func NewHttpClient() *HttpClient {
	conf := LoadConfig()
	return &HttpClient{baseUrl: conf.BaseUrl}
}

func (c *HttpClient) printUsage() {
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

func (c *HttpClient) Run() {
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

func (c *HttpClient) send(from, to, amount string, fee uint64) {
	inputAddrs := strings.Split(from, ",")
	outputAddrs := strings.Split(to, ",")
	amounts := strings.Split(amount, ",")

	if len(outputAddrs) != len(amounts) {
		log.Fatal("输出地址与输出金额没有匹配到")
	}

	for _, addr := range outputAddrs {
		if !utils.IsValidAddress(addr) {
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
		if !utils.IsValidAddress(addr) {
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

	vins := make([]blc.TxInput, 0)
	for _, addr := range inputAddrs {
		utxo := c.getUtxo(addr)
		if utxo == nil {
			log.Fatalf("no utxo with address %s", addr)
		}
		wallet, err := mgr.GetWallet(addr)
		if err != nil {
			log.Fatal("not found wallet:", err)
		}
		for txHash, outputs := range utxo {
			for index, _ := range outputs {
				vin, err := blc.NewTxInput(txHash, uint32(index), &wallet.Key)
				if err != nil {
					log.Fatal("create tx vin failed:", err)
				}
				vins = append(vins, *vin)
			}
		}
	}

	vouts := make([]blc.TxOutput, 0)
	for i, addr := range outputAddrs {
		vout, err := blc.NewTxOutput(uint64Amounts[i], addr)
		if err != nil {
			log.Fatal("create tx vout failed:", err)
		}
		vouts = append(vouts, *vout)
	}

	if inputAmount > (outputAmount + fee) {
		log.Info("找零: ", inputAmount-(outputAmount+fee))
		// 找零
		addrs := mgr.Addresses()
		rand.Seed(time.Now().UnixNano())
		addr := addrs[rand.Intn(len(addrs))] // 找零地址
		vout, err := blc.NewTxOutput(inputAmount-(outputAmount+fee), addr)
		if err != nil {
			log.Fatal("create tx vout failed:", err)
		}
		vouts = append(vouts, *vout)
	}

	tx, err := blc.NewTransaction(vins, vouts)
	if err != nil {
		log.Fatal("create transaction failed:", err)
	}

	if false == c.postTx(tx) {
		log.Fatal("交易提交失败")
	}

	log.Info("交易提交成功")
}

func (c *HttpClient) printAllBalance() {
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

func (c *HttpClient) printBalance(address string) {
	log.Info("balance: ", c.getBalance(address))
}

func (c *HttpClient) printTxPool() {
	data, err := c.Get("/txPool", []byte{})
	if err != nil {
		log.Fatal(err)
	}
	pool := blc.TxPool{}
	if err = json.Unmarshal(data, &pool); err != nil {
		log.Fatal(err)
	}
	pool.Traverse(func(fee uint64, tx *blc.Transaction) bool {
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

func (c *HttpClient) printHeight() {
	log.Info("height: ", c.getHeight())
}

func (c *HttpClient) printChain() {
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

func (c *HttpClient) printAddresses() {
	mgr := walletmgr.New()
	fmt.Println("地址列表：")
	for _, address := range mgr.Addresses() {
		fmt.Printf("\t%s\n", address)
	}
}

func (c *HttpClient) removeWallet(address string) {
	mgr := walletmgr.New()
	err := mgr.RemoveWallet(address)
	if err != nil {
		log.Warn(err)
		return
	}
	log.Info("remove wallet success")
}

func (c *HttpClient) createWallet() {
	mgr := walletmgr.New()
	w, err := mgr.CreateWallet()
	if err != nil {
		log.Fatal("create wallet failed: ", err)
	}
	log.Infof("create wallet success, address=%s", w.GetAddress())
}

func (c *HttpClient) postTx(tx *blc.Transaction) bool {
	data, err := tx.Marshal()
	if err != nil {
		log.Fatal(err)
	}

	_, err = c.Post("/tx", data)
	if err != nil {
		log.Error(err)
		return false
	}

	return true
}

func (c *HttpClient) getUtxo(address string) map[[32]byte]map[int]*blc.TxOutput {
	type Request struct {
		Address string `json:"address"`
	}
	req := Request{Address: address}
	data, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	body, err := c.Get("/utxo", data)
	if err != nil {
		log.Fatal(err)
	}

	type Utxo struct {
		TxHash [32]byte
		Index  int
		TxOut  blc.TxOutput
	}
	utxoes := make([]Utxo, 0)

	if err = json.Unmarshal(body, &utxoes); err != nil {
		log.Fatal(err)
	}

	if len(utxoes) == 0 {
		return nil
	}

	ret := map[[32]byte]map[int]*blc.TxOutput{}
	for _, utxo := range utxoes {
		if _, ok := ret[utxo.TxHash]; !ok {
			ret[utxo.TxHash] = map[int]*blc.TxOutput{}
		}
		ret[utxo.TxHash][utxo.Index] = &utxo.TxOut
	}

	return ret
}

func (c *HttpClient) getBalance(address string) uint64 {
	type Request struct {
		Address string `json:"address"`
	}
	req := Request{Address: address}
	data, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}

	body, err := c.Get("/balance", data)
	if err != nil {
		log.Fatal(err)
	}

	type Response struct {
		Balance uint64 `json:"balance"`
	}
	resp := Response{}
	if err = json.Unmarshal(body, &resp); err != nil {
		log.Fatal(err)
	}

	return resp.Balance
}

func (c *HttpClient) getBlocks(start, end uint64) []blc.Block {
	type Request struct {
		Start uint64 `json:"start"`
		End   uint64 `json:"end"`
	}
	req := Request{Start: start, End: end}
	data, _ := json.Marshal(req)

	body, err := c.Get("/blocks", data)
	if err != nil {
		log.Fatal(err)
	}

	type Response struct {
		Blocks []blc.Block `json:"blocks"`
	}
	resp := Response{}
	if err = json.Unmarshal(body, &resp); err != nil {
		log.Fatal(err)
	}

	return resp.Blocks
}

func (c *HttpClient) getHeight() uint64 {
	data, err := c.Get("/height", []byte{})
	if err != nil {
		log.Fatal(err)
	}
	type Response struct {
		Height uint64 `json:"height"`
	}
	resp := Response{}
	if err = json.Unmarshal(data, &resp); err != nil {
		log.Fatal(err)
	}
	return resp.Height
}

func (c *HttpClient) Url(route string) string {
	url := fmt.Sprintf("%s%s", c.baseUrl, route)
	return url
}

func (c *HttpClient) Get(route string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("GET", c.Url(route), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create get request failed:%v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.Do(req)
}

func (c *HttpClient) Post(route string, body []byte) ([]byte, error) {
	req, err := http.NewRequest("POST", c.Url(route), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create post request failed:%v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	return c.Do(req)
}

func (c *HttpClient) Do(req *http.Request) ([]byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get failed: %v", err)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read resp body failed:%v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status code:%d body:%s", resp.StatusCode, string(b))
	}

	return b, nil
}
