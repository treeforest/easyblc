package gob

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
)

func EncodeP256(e interface{}) ([]byte, error) {
	gob.Register(elliptic.P256())
	return Encode(e)
}

func Encode(e interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(e); err != nil {
		return nil, fmt.Errorf("gob encode failed: [%v] [%v]", e, err)
	}
	return buf.Bytes(), nil
}

func DecodeP256(data []byte, o interface{}) error {
	gob.Register(elliptic.P256())
	return Decode(data, o)
}

func Decode(data []byte, o interface{}) error {
	dec := gob.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(o); err != nil {
		return fmt.Errorf("gob decode failed: data[%v] obj[%v] error[%v]", data, o, err)
	}
	return nil
}
