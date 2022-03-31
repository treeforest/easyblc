package bloom

import "github.com/bits-and-blooms/bloom/v3"

func New() *bloom.BloomFilter {
	return bloom.New(100000, 4)
}
