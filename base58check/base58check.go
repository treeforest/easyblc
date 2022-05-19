package base58check

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"github.com/treeforest/easyblc/base58"
)

const (
	version = byte(0x00)
)

func Encode(hash160 []byte) (base58checkEncoded []byte) {
	encoded := make([]byte, len(hash160)+1)
	encoded[0] = version
	copy(encoded[1:], hash160)

	// 执行两次 SHA-256
	hash := sha256.Sum256(encoded)
	hash2 := sha256.Sum256(hash[:])

	checksum := hash2[0:4]
	encodedChecksum := append(encoded, checksum...)
	base58EncodedChecksum := base58.Encode(encodedChecksum)

	// 由于base58会将0删除，比特币要求在0的位置补上1，即比特币地址由1开始的原因
	var buffer bytes.Buffer
	for _, v := range encodedChecksum {
		if v != byte(0x00) {
			break
		}
		buffer.WriteByte('1')
	}
	buffer.Write(base58EncodedChecksum)

	return buffer.Bytes()
}

func Decode(b []byte) (hash160 []byte, err error) {
	encodedChecksum := base58.Decode(b[1:])

	hash160 = encodedChecksum[:len(encodedChecksum)-4]
	checksum := encodedChecksum[len(encodedChecksum)-4:]

	var buffer bytes.Buffer
	for _, v := range b {
		if v != '1' {
			break
		}
		buffer.WriteByte(0)
	}
	buffer.Write(hash160)
	encoded := buffer.Bytes()

	// 执行两次 SHA-256,验证校验码是否正确
	hash := sha256.Sum256(encoded)
	hash2 := sha256.Sum256(hash[:])

	if !bytes.Equal(hash2[:4], checksum) {
		return nil, fmt.Errorf("checksum error")
	}

	return hash160, nil
}
