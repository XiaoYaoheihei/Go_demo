package server

import (
	"Minymq/protocol"
	"io"
	"log"
)

// 处理和消费者之间的TCP连接情况,充当个“代理人”的角色
type Broker struct {
	//包含连接和状态字段
	conn  io.ReadWriteCloser
	name  string
	state int
}

// 返回Broker
func NewBroker(conn io.ReadWriteCloser, name string) *Broker {
	return &Broker{conn, name, -1}
}

// 关闭连接
func (b *Broker) Close() {
	log.Printf("broker(%s) is closing", b.String())
	b.conn.Close()
}

func (b *Broker) String() string {
	return b.name
}
func (b *Broker) GetState() int {
	return b.state
}
func (b *Broker) SetState(state int) {
	b.state = state
}

func (b *Broker) Read(data []byte) (int, error) {
	return b.conn.Read(data)
}

// Write方法
func (b *Broker) Write(data []byte) (int, error) {

}

// 从客户端读取信息，设置状态，回复
func (b *Broker) Handle() {
	defer b.Close()
	proto := &protocol.Protocol{}
	err := proto.IoLoop(b)
	if err != nil {
		log.Printf("error: broker(%s) - %s", b.String(), err.Error())
		return
	}
}
