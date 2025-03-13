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

func (c *JsonCodec) Decode(data []byte, msg *any) error {
	err := json.Unmarshal(data, msg)
	if err != nil {
		slog.Error("json codec:", "json decode error", err)
	}
	return err
}
