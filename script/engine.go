package script

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"github.com/treeforest/easyblc/internal/blc/script/stack"
	"golang.org/x/crypto/ripemd160"
)

// Engine 基于堆栈的脚本执行引擎
type Engine struct {
	Ops    []Op
	TxHash []byte
}

func (e *Engine) Run() bool {
	st := stack.New()      // 栈
	step := len(e.Ops) * 2 // 最大运行次数为操作数的2倍

	for !e.empty() && step > 0 {
		step--
		op := e.pop()

		switch op.Code {
		case PUSH:
			st.Push(op.Data)
		case DUP:
			st.Push(st.Top())
		case HASH160:
			if !st.Has(1) {
				return false
			}
			data := st.Pop()
			sha := sha256.Sum256(data)
			r := ripemd160.New()
			r.Write(sha[:])
			hash160 := r.Sum(nil)
			st.Push(hash160)
		case EQUALVERIFY:
			if !st.Has(2) {
				return false
			}
			a := st.Pop()
			b := st.Pop()
			if !bytes.Equal(a, b) {
				return false
			}
		case CHECKSIG:
			if !st.Has(2) {
				return false
			}
			pubData := st.Pop()
			sig := st.Pop()
			pub := ecdsa.PublicKey{Curve: elliptic.P256()}
			pub.X, pub.Y = elliptic.Unmarshal(pub.Curve, pubData)
			if pub.X == nil || pub.Y == nil {
				return false
			}
			if ok := ecdsa.VerifyASN1(&pub, e.TxHash, sig); !ok {
				return false
			}
		default:
			// unknown op code
			return false
		}
	}
	if step <= 0 {
		return false
	}
	return true
}

func (e *Engine) push(ops []Op) {
	e.Ops = append(e.Ops, ops...)
}

func (e *Engine) pop() Op {
	l := len(e.Ops)
	op := e.Ops[l-1]
	e.Ops = e.Ops[:l-1]
	return op
}

func (e *Engine) empty() bool {
	return len(e.Ops) == 0
}
