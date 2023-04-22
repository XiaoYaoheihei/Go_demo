package server

import (
	"context"
	"log"
	"net"
)

func TcpServer(ctx context.Context, addr, port string) {
	address := addr + ":" + port
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic("tcp listen(" + address + ") failed")
	}
	log.Printf("listening for clients on %s", address)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			//每当监听到tcp连接，就新建一个broker来处理
			conn, err := listener.Accept()
			if err != nil {
				panic("accept failed: " + err.Error())
			}
			broker := NewBroker(conn, conn.RemoteAddr().String())
			go broker.Handle(ctx)
		}
	}
}
