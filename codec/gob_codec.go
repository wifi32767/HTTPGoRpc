package codec

import (
	"bytes"
	"encoding/gob"
)

type GobCodec struct {
	buffer bytes.Buffer
	reader *bytes.Reader
}

func NewGobCodec() Codec {
	return &GobCodec{
		buffer: bytes.Buffer{},
		reader: &bytes.Reader{},
	}
}

func (g *GobCodec) Encode(msg any) ([]byte, error) {
	g.buffer.Reset()
	encoder := gob.NewEncoder(&g.buffer)
	err := encoder.Encode(msg)
	if err != nil {
		return nil, err
	}
	return g.buffer.Bytes(), nil
}

func (g *GobCodec) Decode(data []byte, msg *any) error {
	g.reader.Reset(data)
	decoder := gob.NewDecoder(g.reader)
	err := decoder.Decode(msg)
	return err
}
