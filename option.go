package gorpc

import (
	"github.com/wifi32767/HTTPGoRpc/codec"
)

const MagicNumber int = 0x123456

const (
	TypeCall     = "Call"
	TypeRegister = "Reg"
	TypePing     = "Ping"
	TypeAsk      = "Ask"
)

type Header struct {
	Service string
	Method  string
	Option  Options
}

type Options struct {
	MagicNumber int
	CodecType   codec.Type
	UseRegistry bool
}

var DefaultOptions = &Options{
	MagicNumber: MagicNumber,
	CodecType:   codec.TypeGob,
	UseRegistry: false,
}
