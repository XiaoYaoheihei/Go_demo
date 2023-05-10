package geerpc

import (
	"encoding/json"
	"fmt"
	"geerpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

const MagicNumber = 0x3bef5c

// 协商消息的编解码方式
type Option struct {
	MagicNumber int
	CodecType   codec.Type
}

// 默认方式
var DefaultOption = &Option{
	MagicNumber: MagicNumber,
	CodecType:   codec.Type(codec.GobType),
}

type Server struct {
}

func NewServer() *Server {
	return &Server{}
}

var DefaultServer = NewServer()

// 默认建立连接
func Accept(listner net.Listener) {
	DefaultServer.Accept(listner)
}

func (s *Server) Accept(listner net.Listener) {
	for {
		//for循环等待socket连接建立
		//返回一个连接对象
		conn, err := listner.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return
		}
		go s.ServeConn(conn)
	}
}

func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	defer func() {
		_ = conn.Close()
	}()
	//首先使用json的反序列化得到Option实例，检查里面的值是否正确
	//然后根据CodeType得到对应的消息编码器
	var op Option
	if err := json.NewDecoder(conn).Decode(&op); err != nil {
		log.Println("rpc server: options error: ", err)
		return
	}
	if op.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", op.MagicNumber)
		return
	}
	f := codec.NewCodecMap[op.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", op.CodecType)
		return
	}
	//检查完毕之后进行自己的处理逻辑
	s.serveCodec(f(conn))
}

var invalidRequest = struct{}{}

func (s *Server) serveCodec(cc codec.Codec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	//无限制地等待请求的到来，直到发生错误
	for {
		//读取请求
		req, err := s.readRequest(cc)
		if err != nil {
			if req == nil {
				break
			}
			req.h.Error = err.Error()
			s.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		//协程处理请求
		go s.handleRequest(cc, req, sending, wg)
	}
	wg.Wait()
	_ = cc.Close()
}

type requeset struct {
	//请求头
	h            *codec.Header
	argv, replyv reflect.Value
}

func (s *Server) readRequest(cc codec.Codec) (*requeset, error) {
	//读取请求头
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	//根据请求头封装请求信息
	req := &requeset{h: h}
	//目前我们并不清楚argv的类型，简单只是返回string
	req.argv = reflect.New(reflect.TypeOf(""))
	if err = cc.ReadBody(req.argv.Interface()); err != nil {
		log.Println("rpc server: read argv err:", err)
	}
	return req, nil
}

func (s *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && err != io.ErrUnexpectedEOF {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (s *Server) handleRequest(cc codec.Codec, req *requeset, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	log.Println(req.h, req.argv.Elem())
	req.replyv = reflect.ValueOf(fmt.Sprintf("geerpc resp %d", req.h.Seq))
	s.sendResponse(cc, req.h, req.replyv.Interface(), sending)
}

func (s *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	//锁保证顺序地返回值，而不会导致写入的顺序发生改变
	sending.Lock()
	defer sending.Unlock()
	//向客户端写入值
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}
