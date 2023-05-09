package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

// Gob对象
type GobCodec struct {
	conn io.ReadWriteCloser
	buf  *bufio.Writer
	dec  *gob.Decoder
	enc  *gob.Encoder
}

var _ Codec = (*GobCodec)(nil)

// 构造函数
// conn 是由构建函数传入
// 通常是通过 TCP 或者 Unix 建立 socket 时得到的链接实例
func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn: conn,
		buf:  buf,
		dec:  gob.NewDecoder(conn),
		enc:  gob.NewEncoder(buf),
	}
}

// 解码header和body部分
func (g *GobCodec) ReadHeader(h *Header) error {
	return g.dec.Decode(h)
}

func (g *GobCodec) ReadBody(body interface{}) error {
	return g.dec.Decode(body)
}

func (g *GobCodec) Writer(h *Header, body interface{}) (err error) {
	defer func() {
		//清空缓冲区
		_ = g.buf.Flush()
		//出现异常关闭连接
		if err != nil {
			_ = g.Close()
		}
	}()
	//分别编码header和body部分
	if err := g.enc.Encode(h); err != nil {
		log.Println("rpc codec:gob error encoding header:", err)
		return err
	}
	if err := g.enc.Encode(body); err != nil {
		log.Println("rpc codec: gob error encoding body:", err)
		return err
	}
	return nil
}

func (g *GobCodec) Close() error {
	return g.conn.Close()
}
