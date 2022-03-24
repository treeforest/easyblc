package base58

import (
	"bytes"
	"math/big"
)

const (
	// base58 编码基数表
	alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
)

func Encode(b []byte) []byte {
	var (
		x     *big.Int // b对应的大整数
		radix *big.Int // 除数
		zero  *big.Int // 大整数0
		mod   *big.Int // 模
		dst   []byte   // 编码结果
	)

	x = big.NewInt(0).SetBytes(b)
	radix = big.NewInt(58) // 58
	zero = big.NewInt(0)
	mod = &big.Int{}
	for x.Cmp(zero) != 0 {
		x.DivMod(x, radix, mod) // 除余法
		dst = append(dst, alphabet[mod.Int64()])
	}

	reverse(dst)
	return dst
}

func Decode(b []byte) []byte {
	r := big.NewInt(0)
	for _, c := range b {
		i := bytes.IndexByte([]byte(alphabet), c)
		r.Mul(r, big.NewInt(58))
		r.Add(r, big.NewInt(int64(i)))
	}
	return r.Bytes()
}

func reverse(b []byte) {
	i, j := 0, len(b)-1
	for i < j {
		b[i], b[j] = b[j], b[i]
		i++
		j--
	}
}
