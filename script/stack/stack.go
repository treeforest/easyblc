package stack

type stack struct {
	l [][]byte
}

func New() *stack {
	return &stack{l: make([][]byte, 0)}
}

func (s *stack) Push(v []byte) {
	s.l = append(s.l, v)
}

func (s *stack) Pop() []byte {
	v := s.l[len(s.l)-1]
	s.l = s.l[:len(s.l)-1]
	return v
}

func (s *stack) Top() []byte {
	return s.l[len(s.l)-1]
}

func (s *stack) Empty() bool {
	return s.Len() == 0
}

func (s *stack) Len() int {
	return len(s.l)
}

func (s *stack) Has(n int) bool {
	return len(s.l) >= n
}
