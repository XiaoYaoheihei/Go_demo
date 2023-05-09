package codec

import "io"

type Header struct {
	ServiceMethod string
	Seq           uint64
	Error         string
}

// 抽象类，方便不同的对象不同的编解码方式
type Codec interface {
	io.Closer
	ReadHeader(*Header) error
	ReadBody(interface{}) error
	Writer(*Header, interface{}) error
}

// 抽象出Codec的构造函数
type NewCodecFunc func(writer io.ReadWriter) Codec

const (
	GobType string = "application/gob"
)

// map用来存储具体的对应关系
var NewCodecMap map[string]NewCodecFunc

func init() {
	NewCodecMap = make(map[string]NewCodecFunc)
	NewCodecMap[GobType] = NewGobCodec
}
