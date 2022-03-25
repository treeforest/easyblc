package script

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"github.com/treeforest/easyblc/pkg/base58check"
	"github.com/treeforest/easyblc/pkg/gob"
	log "github.com/treeforest/logger"
)

func ParsePubKeyHashFromScriptSig(scriptSig []byte) ([]byte, error) {
	var ops []Op
	err := gob.Decode(scriptSig, &ops)
	return ops[0].Data, err
}

func ParsePubKeyHashFromScriptPubKey(scriptPubKey []byte) ([]byte, error) {
	var ops []Op
	err := gob.Decode(scriptPubKey, &ops)
	return ops[2].Data, err
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

// IsValidScriptPubKey 是否是有效的交易输入脚本
func IsValidScriptPubKey(scriptPubKey []byte) bool {
	var ops []Op
	err := gob.Decode(scriptPubKey, &ops)
	if err != nil {
		log.Debug("decode script pub key error:", err)
		return false
	}
	if len(ops) != 5 {
		log.Debug("script pub key sum error")
		return false
	}
	codes := []OPCODE{CHECKSIG, EQUALVERIFY, PUSH, HASH160, DUP}
	requireNull := []bool{true, true, false, true, true}
	for i, op := range ops {
		if op.Code != codes[i] {
			log.Debugf("opcode error, require:%d actually:%d", op.Code, codes[i])
			return false
		}
		if requireNull[i] {
			if op.Data != nil {
				log.Debugf("%d data should be null", i)
				return false
			}
		} else {
			if op.Data == nil || len(op.Data) == 0 {
				log.Debugf("%d data should be not null", i)
				return false
			}
		}
	}
	return true
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

// IsValidScriptSig 是否是有效的交易输入脚本
func IsValidScriptSig(scriptSig []byte) bool {
	var ops []Op
	err := gob.Decode(scriptSig, &ops)
	if err != nil {
		log.Debug("decode scriptsig failed:", err)
		return false
	}
	if len(ops) != 2 {
		log.Debug("opcode sum error")
		return false
	}
	for _, op := range ops {
		if op.Code != PUSH || op.Data == nil || len(op.Data) == 0 {
			log.Debug("opcode format error")
			return false
		}
	}
	return true
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
