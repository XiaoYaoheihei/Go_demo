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

type Type string

const (
	GobType string = "application/gob"
)

// map用来存储具体的对应关系
var NewCodecMap map[Type]NewCodecFunc

func init() {
	NewCodecMap = make(map[Type]NewCodecFunc)
	NewCodecMap[GobType] = NewGobCodec
}
