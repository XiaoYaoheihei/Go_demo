package geerpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"log"
	"net"
	"sync"
	"time"
)

type Call struct {
	Seq uint64
	//"<service>.<method>"
	ServiceMethod string
	//函数参数
	Args interface{}
	//函数的返回值
	Reply interface{}
	Error error
	//该字段是为了支持异步调用
	Done chan *Call
}

func (c *Call) done() {
	c.Done <- c
}

type Client struct {
	cc       codec.Codec
	opt      *Option
	sending  sync.Mutex
	header   codec.Header
	mu       sync.Mutex
	seq      uint64
	pending  map[uint64]*Call
	closing  bool
	shutdown bool
}

// 客户端回应
type clientResults struct {
	client *Client
	err    error
}

type newClientFunc func(conn net.Conn, opt *Option) (client *Client, err error)

var ErrShutdown = errors.New("connection is shut down")

func parse(opts ...*Option) (*Option, error) {
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}
	//取opts切片
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

// 为Dial增加一层超时处理
func dialTimeout(f newClientFunc, network, address string, opts ...*Option) (client *Client, err error) {
	//将options解析
	opt, err := parse(opts...)
	if err != nil {
		return nil, err
	}
	//如果连接创建超时，将返回错误
	conn, err := net.DialTimeout(network, address, opt.ConnectTimeout)
	if err != nil {
		return nil, err
	}
	//如果客户端是空的，记得关闭连接
	defer func() {
		if client == nil {
			_ = conn.Close()
		}
	}()
	ch := make(chan clientResults)
	go func() {
		//使用子协程执行NewClient，执行完之后发送结果到ch中
		client, err := f(conn, opt)
		ch <- clientResults{client: client, err: err}
	}()
	select {
	//如果他首先接收到消息，就说明执行NewClient执行超时，返回错误
	case <-time.After(opt.ConnectTimeout):
		return nil, fmt.Errorf("rpc client: connect timeout: expect within %s", opt.ConnectTimeout)
	case results := <-ch:
		return results.client, results.err
	}
}

// 方便用户传入服务端地址，创建Client实例
func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	return dialTimeout(NewClient, network, address, opts...)
}

// 创建client实例
func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	//根据opt中的codectype确定本次的客户端编解码对象
	f := codec.NewCodecMap[opt.CodecType]
	if f == nil {
		err := fmt.Errorf("invalid codec type %s", opt.CodecType)
		log.Println("rpc client: codec error:", err)
		return nil, err
	}
	//完成一开始的协议交换，协商好消息的编解码方式
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error: ", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn), opt), nil
}

func newClientCodec(cc codec.Codec, opt *Option) *Client {
	client := &Client{
		//从1开始
		seq:     1,
		cc:      cc,
		opt:     opt,
		pending: make(map[uint64]*Call),
	}
	//开启子协程接收响应
	go client.receive()
	return client
}

// 客户端关闭
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

// 检查客户端是否活着
func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

// 将参数 call 添加到 client.pending 中，并更新 client.seq
func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = c.seq
	c.pending[call.Seq] = call
	//序号++
	c.seq++
	return call.Seq, nil
}

// 根据 seq，从 client.pending 中移除对应的 call，并返回
func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// 服务端或客户端发生错误时调用，将 shutdown 设置为 true，
// 且将错误信息通知所有 pending 状态的 call。
func (c *Client) terminateCalls(err error) {
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		//异步返回值
		call.done()
	}
}

// 客户端接收响应
func (c *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = c.cc.ReadHeader(&h); err != nil {
			break
		}
		call := c.removeCall(h.Seq)
		switch {
		//call不存在
		case call == nil:
			err = c.cc.ReadBody(nil)
		//call存在，但是服务端处理错误
		case h.Error != "":
			call.Error = fmt.Errorf(h.Error)
			err = c.cc.ReadBody(nil)
			call.done()
		//call存在，服务端处理正常，需要从body中读取reply的值
		default:
			err = c.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body" + err.Error())
			}
			call.done()
		}
	}
	//客户端发送错误，需将错误发送给pending
	c.terminateCalls(err)
}

// 客户端发送请求
func (c *Client) send(call *Call) {
	//确保一个请求能够完整的发送
	c.sending.Lock()
	defer c.sending.Unlock()
	//注册调用
	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	//准备请求头
	c.header.ServiceMethod = call.ServiceMethod
	c.header.Seq = seq
	c.header.Error = ""
	//编码并且发送请求
	if err := c.cc.Write(&c.header, call.Args); err != nil {
		//通过序号移除调用
		call := c.removeCall(seq)
		//如果call是nil的话，就证明write部分失败，客户端已经收到响应并且处理
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// 对Go的封装，阻塞call.Done,等待响应返回，是一个同步接口
func (c *Client) Call(ctx context.Context, seviceMethod string, args, reply interface{}) error {
	//调用命名函数，等待完成
	call := c.Go(seviceMethod, args, reply, make(chan *Call, 1))
	select {
	case <-ctx.Done():
		c.removeCall(call.Seq)
		return errors.New("rpc client: call failed: " + ctx.Err().Error())
	case call := <-call.Done:
		return call.Error
	}

}

// 对外提供了异步调用的接口
func (c *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	c.send(call)
	return call
}
