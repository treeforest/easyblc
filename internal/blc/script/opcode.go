package script

type OPCODE byte

const (
	DUP OPCODE = iota + 10
	PUSH
	HASH160
	EQUALVERIFY
	CHECKSIG
)

// Op 操作码
type Op struct {
	Code OPCODE
	Data []byte
}
