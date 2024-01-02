package protocal

import "io"

//定义一些客户端的错误以及需要用到的常亮和接口

const (
	ClientInit = iota
	ClientWaitGet
	ClientWaitResponse
)

type ClientError struct {
	errStr string
}

// 定义一些常见的错误
var (
	Invalid    = ClientError{errStr: "E_INVALID"}
	BadTopic   = ClientError{errStr: "E_BAD_TOPIC"}
	BadChannel = ClientError{errStr: "E_BAD_CHANNEL"}
	BadMessage = ClientError{errStr: "E_BAD_MESSAGE"}
)

func (c ClientError) Error() string {
	return c.errStr
}

type StateReadWrite interface {
	io.ReadWriter
	GetState() int
	SetState(state int)
	String() string
}
