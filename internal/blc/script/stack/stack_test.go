package stack

import (
	"github.com/stretchr/testify/require"
	"strconv"
	"testing"
)

func TestStack(t *testing.T) {
	s := New()

	for i := 10; i < 20; i++ {
		s.Push([]byte(strconv.Itoa(i)))
	}

	for !s.Empty() {
		v := s.Pop()
		t.Log(string(v))
	}

	require.Equal(t, s.Len(), 0)
}
