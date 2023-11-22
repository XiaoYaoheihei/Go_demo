package server

import (
	"encoding/binary"
	"io"
	"log"
)

//消费端的处理流程

type Client struct {
	name  string
	state int
	conn  io.ReadWriteCloser //维护的连接
}

func NewClient(conn io.ReadWriteCloser, name string) *Client {
	return &Client{
		name:  name,
		state: -1,
		conn:  conn,
	}
}

func (c *Client) Read(data []byte) (int, error) {
	return c.conn.Read(data)
}

func (c *Client) Write(data []byte) (int, error) {
	//先以二进制大端的形式写入长度
	err := binary.Write(c.conn, binary.BigEndian, int32(len(data)))
	if err != nil {
		return 0, err
	}

	n, err := c.conn.Write(data)
	if err != nil {
		return 0, err
	}
	return n + 4, nil
}

func (c *Client) Close() {
	log.Printf("CLIENT(%s): closing", c.String())
	c.conn.Close()
}

func (c *Client) String() string {
	return c.name
}

func (c *Client) Getstate() int {
	return c.state
}

func (c *Client) Getname() string {
	return c.name
}

func (c *Client) Setstate(state int) {
	c.state = state
}
