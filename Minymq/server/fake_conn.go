// 伪造的客户端向协议中发送消息
package server

import "io"

type FakeConn struct {
	io.ReadWriter
}

func (f *FakeConn) Close() error {
	return nil
}
