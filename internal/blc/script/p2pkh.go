package script

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/treeforest/easyblc/pkg/base58check"
	"github.com/treeforest/easyblc/pkg/gob"
)

func ParsePubKeyHashFromScriptSig(scriptSig []byte) []byte {
	var ops []Op
	gob.Decode(scriptSig, ops)
	return ops[0].Data
}

func ParsePubKeyHashFromScriptPubKey(scriptPubKey []byte) []byte {
	var ops []Op
	gob.Decode(scriptPubKey, ops)
	return ops[2].Data
}

// GenerateScriptPubKey 生成交易输出脚本
func GenerateScriptPubKey(address []byte) ([]byte, error) {
	hash160, err := base58check.Decode(address)
	if err != nil {
		return nil, err
	}
	scriptPubKey := []Op{
		{Code: CHECKSIG, Data: nil}, // 栈底
		{Code: EQUALVERIFY, Data: nil},
		{Code: PUSH, Data: hash160},
		{Code: HASH160, Data: nil},
		{Code: DUP, Data: nil}, // 栈顶
	}
	return gob.Encode(scriptPubKey)
}

// GenerateScriptSig 生成交易输入脚本
func GenerateScriptSig(txHash []byte, key *ecdsa.PrivateKey) ([]byte, error) {
	// 对交易哈希的签名
	sig, err := ecdsa.SignASN1(rand.Reader, key, txHash)
	if err != nil {
		return nil, fmt.Errorf("sign failed: %v", err)
	}
	pub := elliptic.Marshal(elliptic.P256(), key.PublicKey.X, key.PublicKey.Y)

	// 将签名和公钥拼接成交易输入脚本
	scriptSig := []Op{
		{Code: PUSH, Data: pub},
		{Code: PUSH, Data: sig},
	}

	return gob.Encode(scriptSig)
}

// Verify 验证输入脚本是否可消费输出脚本的金额
func Verify(txHash, input, output []byte) bool {
	var in, out []Op
	gob.Decode(input, &in)
	gob.Decode(output, &out)
	e := Engine{Ops: make([]Op, 0), TxHash: txHash}
	e.push(out)
	e.push(in)
	return e.Run()
}
