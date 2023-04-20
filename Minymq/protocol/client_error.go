// 客户端的一些错误以及常用到的常量和接口
package protocol

import "io"

// 三种常量
const (
	ClientInit = iota
	ClientWaitGet
	ClientWaitResponse
)

type StatefulReadWriter interface {
	io.ReadWriter
	GetState() int
	SetState(state int)
	String() string
}
type ClientError struct {
	errStr string
}

func (ce ClientError) Error() string {
	return ce.errStr
}

// 错误的变量类型
var (
	ClientErrInvalid    = ClientError{"E_INVALID"}
	ClientErrBadTopic   = ClientError{"E_BAD_TOPIC"}
	ClientErrBadChannel = ClientError{"E_BAD_CHANNEL"}
	ClientErrBadMessage = ClientError{"E_BAD_MESSAGE"}
)
