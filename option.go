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
	Service string // 服务名
	Method  string // 方法名
	Option  Options
}

type Options struct {
	MagicNumber int        // 验证传输正确性的魔数
	CodecType   codec.Type // 编解码器类型
	UseRegistry bool       // 是否使用注册中心
}

var DefaultOptions = &Options{
	MagicNumber: MagicNumber,
	CodecType:   codec.TypeGob,
	UseRegistry: false,
}
