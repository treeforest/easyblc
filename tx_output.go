package blc

import (
	"fmt"
	"github.com/treeforest/easyblc/script"
)

type TxOutput struct {
	Value        uint64 // 金额
	Address      string // 地址
	ScriptPubKey []byte // 锁定脚本
}

func NewTxOutput(value uint64, address string) (*TxOutput, error) {
	out := &TxOutput{
		Value:        value,
		Address:      address,
		ScriptPubKey: nil,
	}
	err := out.GenScriptPubKey()
	return out, err
}

func (out *TxOutput) GenScriptPubKey() error {
	scriptPubKey, err := script.GenerateScriptPubKey([]byte(out.Address))
	if err != nil {
		return fmt.Errorf("generate script public key failed:%v", err)
	}
	out.ScriptPubKey = scriptPubKey
	return nil
}
