package codec

import (
	"encoding/json"
	"log/slog"
)

type JsonCodec struct {
}

func NewJsonCodec() Codec {
	return &JsonCodec{}
}

func (c *JsonCodec) Encode(msg any) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("json codec:", "json encode error", err)
	}
	return data, err
}

func (c *JsonCodec) EncodeString(msg any) (string, error) {
	data, err := c.Encode(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *JsonCodec) Decode(data []byte, msg any) error {
	err := json.Unmarshal(data, msg)
	if err != nil {
		slog.Error("json codec:", "json decode error", err)
	}
	return err
}

func (c *JsonCodec) DecodeString(data string, msg any) error {
	return c.Decode([]byte(data), msg)
}
