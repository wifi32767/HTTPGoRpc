package gorpc

import (
	"github.com/wifi32767/GoRpc/codec"
)

const MagicNumber int = 0x123456

const (
	TypeCall    = "Call"
	TypeConnect = "Connect"
)

type Header struct {
	Service string
	Method  string
	Option  Options
}

type Options struct {
	MagicNumber int
	CodecType   codec.Type
}

var DefaultOptions = &Options{
	MagicNumber: MagicNumber,
	CodecType:   codec.TypeGob,
}
