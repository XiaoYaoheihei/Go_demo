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
	Write(*Header, interface{}) error
}

// 抽象出Codec的构造函数
// 返回的是一个具体的对象
type NewCodecFunc func(writer io.ReadWriteCloser) Codec

type Type string

const (
	GobType  Type = "application/gob"
	JsonType Type = "application/json"
)

// map用来存储具体的对应关系
var NewCodecMap map[Type]NewCodecFunc

func init() {
	NewCodecMap = make(map[Type]NewCodecFunc)
	//NewGobCodec类型转化成NewCodecFunc
	NewCodecMap[GobType] = NewGobCodec
}
