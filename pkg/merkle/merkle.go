package merkle

import (
	"bytes"
	"crypto/sha256"
	"errors"
	log "github.com/treeforest/logger"
	"sort"
)

type Tree struct {
	MerkleRoot []byte
	Nodes      [][]*Node // 所有的节点
}

type Node struct {
	Parent  *Node  // 父节点
	Brother int    // 兄弟节点(当前层所在的索引)
	Hash    []byte // 哈希
}

func New() *Tree {
	return &Tree{Nodes: make([][]*Node, 0), MerkleRoot: nil}
}

type HashSlices [][]byte

func (s HashSlices) Len() int           { return len(s) }
func (s HashSlices) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s HashSlices) Less(i, j int) bool { return bytes.Compare(s[i], s[j]) == 1 }

// BuildWithHashes 生成默克尔树
func (t *Tree) BuildWithHashes(sha256Hashes [][]byte) []byte {
	if sha256Hashes == nil || len(sha256Hashes) <= 0 {
		log.Fatal("transaction hashed is nil")
	}

	// 有序性
	sort.Sort(HashSlices(sha256Hashes))

	if len(sha256Hashes)%2 != 0 {
		// 若交易为奇数份，拷贝最后一份
		sha256Hashes = append(sha256Hashes, sha256Hashes[len(sha256Hashes)-1])
	}

	// 构建叶子节点
	leafs := make([]*Node, 0, len(sha256Hashes))
	for _, hash := range sha256Hashes {
		leafs = append(leafs, &Node{Parent: nil, Hash: hash})
	}

	t.MerkleRoot = t.build(leafs)
	return t.MerkleRoot
}

// buildBrothers 找朋友，大家一起找朋友~
func buildBrothers(nodes []*Node) {
	var (
		left, right int
		l           = len(nodes)
	)
	for i := 0; i < l; i += 2 {
		left, right = i, i+1
		if right == l {
			nodes[left].Brother = left // 自己是自己的兄弟节点
			continue
		}
		nodes[left].Brother = right
		nodes[right].Brother = left
	}
}

func (t *Tree) build(nodes []*Node) []byte {
	buildBrothers(nodes)
	t.Nodes = append(t.Nodes, nodes)

	var (
		num         int     = len(nodes)
		parents     []*Node = make([]*Node, 0) // 父节点列表
		left, right int
	)

	for i := 0; i < num; i += 2 {
		left, right = i, i+1
		if right == num {
			right = left // 不足偶数份，最后的元素自我复制一份计算哈希
		}

		hash := calculateHash(nodes[left].Hash, nodes[right].Hash)
		parent := &Node{
			Parent: nil,
			Hash:   hash,
		}
		parents = append(parents, parent)
		nodes[left].Parent = parent
		nodes[right].Parent = parent

		if num == 2 {
			// 当只有两个元素的时候，计算出来的节点是根节点（退出条件）
			parent.Brother = 0 // 自己是自己的兄弟节点
			t.Nodes = append(t.Nodes, []*Node{parent})
			return parent.Hash
		}
	}

	return t.build(parents)
}

// calculateHash 计算哈希(按照有序排列数据)
func calculateHash(hash1, hash2 []byte) []byte {
	var data []byte
	if bytes.Compare(hash1, hash2) == -1 {
		data = append(hash1, hash2...)
	} else {
		data = append(hash2, hash1...)
	}
	hash := sha256.Sum256(data)
	return hash[:]
}

// GenerateMerkleProof generate merkel proof
func (t *Tree) GenerateMerkleProof(sha256Hash []byte) ([][]byte, error) {
	var (
		node *Node = nil
	)

	for _, n := range t.Nodes[0] {
		if bytes.Equal(n.Hash, sha256Hash) {
			node = n
			break
		}
	}
	if node == nil {
		return nil, errors.New("target is not a leaf hash")
	}

	var proof [][]byte
	for i := 0; i < len(t.Nodes)-1; i++ {
		brother := t.Nodes[i][node.Brother]
		proof = append(proof, brother.Hash)
		node = node.Parent
	}

	return proof, nil
}

// VerifyMerkleProof verify merkle proof
func (t *Tree) VerifyMerkleProof(txHash []byte, proof [][]byte) bool {
	if t.MerkleRoot == nil || len(t.MerkleRoot) == 0 {
		return false
	}
	var dst []byte = txHash
	for _, b := range proof {
		dst = calculateHash(dst, b)
	}
	if !bytes.Equal(t.MerkleRoot, dst) {
		return false
	}
	return true
}

func VerifyMerkleProof(txHash, merkleRoot []byte, proof [][]byte) bool {
	tree := New()
	tree.MerkleRoot = merkleRoot
	return tree.VerifyMerkleProof(txHash, proof)
}
