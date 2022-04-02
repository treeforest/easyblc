package blc

import (
	"github.com/stretchr/testify/require"
	"github.com/treeforest/easyblc/pkg/gob"
	"testing"
)

func Test(t *testing.T) {
	bs := []*Block{&Block{
		Version: 1,
		Height:  100,
	}}
	data, err := gob.Encode(bs)
	require.NoError(t, err)

	bs2 := make([]*Block, 0)
	err = gob.Decode(data, &bs2)
	require.NoError(t, err)
}
