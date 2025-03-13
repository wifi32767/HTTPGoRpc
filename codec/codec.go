package codec

type Header struct {
	Service string
	Method  string
	SeqId   uint32
	// 之所以这里是string
	// 是因为编解码的时候要使用导出字段
	// errors.errorString是不可导出的
	Error string
}

type Message struct {
	Header *Header
	Body   any
}

// 编解码器，用于将消息体编码成字节流或者将字节流解码成消息体
type Codec interface {
	Encode(any) ([]byte, error)
	Decode(data []byte, msg *any) error
}

type CodecConstructor func() Codec
type Type string
type CodecConstructorMap map[Type]CodecConstructor

const (
	TypeGob  Type = "gob"
	TypeJson Type = "json"
)

var CodecMap = CodecConstructorMap{
	TypeGob:  NewGobCodec,
	TypeJson: NewJsonCodec,
}

// 自定义编解码器
func RegisterCodec(t Type, f CodecConstructor) {
	CodecMap[t] = f
}
