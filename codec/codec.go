package codec

// 编解码器，用于将消息体编码成字节流或者将字节流解码成消息体
type Codec interface {
	Encode(any) ([]byte, error)
	EncodeString(any) (string, error)
	Decode(data []byte, msg *any) error
	DecodeString(data string, msg *any) error
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

func NewCodec(t Type) Codec {
	if f, ok := CodecMap[t]; ok {
		return f()
	}
	return nil
}
