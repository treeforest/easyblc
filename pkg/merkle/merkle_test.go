package merkle

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"github.com/stretchr/testify/require"
	"testing"
)

var calcHashes = func(txs []string) [][]byte {
	hashes := make([][]byte, 0)
	for _, tx := range txs {
		hash := sha256.Sum256([]byte(tx))
		hashes = append(hashes, hash[:])
	}
	return hashes
}

func TestMerkleProof(t *testing.T) {
	tree := New()

	txs := []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}
	hashes := calcHashes(txs)
	merkleRoot := tree.BuildWithHashes(hashes)
	require.NotEmpty(t, merkleRoot)

	txHash := hashes[3]
	proof, err := tree.GenerateMerkleProof(txHash)
	require.NoError(t, err)

	ok := tree.VerifyMerkleProof(txHash, proof)
	require.Equal(t, true, ok)
}

func TestMerkleTreeSorted(t *testing.T) {
	tree01 := New()
	tree02 := New()
	txs01 := []string{"1", "2", "3", "4", "5"}
	txs02 := []string{"1", "5", "3", "4", "2"}
	tree01.BuildWithHashes(calcHashes(txs01))
	tree02.BuildWithHashes(calcHashes(txs02))
	require.Equal(t, tree01.MerkleRoot, tree02.MerkleRoot)
}

func TestEncoder(t *testing.T) {
	tree := New()
	txs := []string{"1", "2", "3", "4"}
	tree.BuildWithHashes(calcHashes(txs))
	var buffer bytes.Buffer
	enc := gob.NewEncoder(&buffer)
	err := enc.Encode(tree)
	require.NoError(t, err)
}
