package base58

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBase58(t *testing.T) {
	src := []byte("this is the example")
	e := Encode(src)
	t.Log(string(e))
	d := Decode(e)
	require.Equal(t, src, d)
}

func BenchmarkEncode(b *testing.B) {
	src := []byte("this is the example")
	for i := 0; i < b.N; i++ {
		_ = Encode(src)
	}
}

func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Decode([]byte("2Cf1ZEY1opMKrSbSgCAYAMw3epujqbUL3Rbg5Tv5omXXUd4qrK"))
	}
}
