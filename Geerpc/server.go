package geerpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"geerpc/codec"
	"go/ast"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const MagicNumber = 0x3bef5c

// 协商消息的编解码方式
type Option struct {
	MagicNumber int
	CodecType   codec.Type
	//超时的相关字段
	ConnectTimeout time.Duration //默认值是10s
	HandleTimeout  time.Duration //默认值是0
}

// 默认方式
var DefaultOption = &Option{
	MagicNumber:    MagicNumber,
	CodecType:      codec.Type(codec.GobType),
	ConnectTimeout: time.Second * 10,
}

// 包含一个方法的完整信息
type methodType struct {
	//方法本身
	method reflect.Method
	//第一个参数的类型
	ArgType reflect.Type
	//第二个参数
	ReplyType reflect.Type
	numCalls  uint64
}

// 某一个服务定义
type service struct {
	//映射的结构体名称
	name string
	//结构体类型，是个interface
	typ reflect.Type
	//结构体实例本身,是个struct
	rcvr reflect.Value
	//存储映射的结构体的所有符合条件的方法
	method map[string]*methodType
}

// 所有的rpc服务定义
type Server struct {
	serviceMap sync.Map
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

// 请求信息中存储了call（调用）的所有信息
type request struct {
	//请求头
	h            *codec.Header
	argv, replyv reflect.Value
	mtype        *methodType
	svc          *service
}

func (s *Server) readRequest(cc codec.Codec) (*request, error) {
	//读取请求头
	h, err := s.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	//根据请求头封装请求信息
	req := &request{h: h}
	req.svc, req.mtype, err = s.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	//通过这两个方法创建出两个参数的实例
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()
	//确保argvi是一个指针类型
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Ptr {
		argvi = req.argv.Addr().Interface()
	}
	//将请求报文反序列化为第一个参数argv
	if err = cc.ReadBody(argvi); err != nil {
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

func (s *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup) {
	defer wg.Done()
	//完成方法调用
	err := req.svc.call(req.mtype, req.argv, req.replyv)
	if err != nil {
		req.h.Error = err.Error()
		s.sendResponse(cc, req.h, invalidRequest, sending)
		return
	}
	//将响应的replyv传递给sendResponse完成序列化
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

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	// arg may be a pointer type, or a value type
	//指针类型和值类型创建实例的方式有区别
	if m.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(m.ArgType.Elem())
	} else {
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType) newReplyv() reflect.Value {
	// reply must be a pointer type
	replyv := reflect.New(m.ReplyType.Elem())
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

// 通过反射值进行调用方法
func (s *service) call(m *methodType, argv, replyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	//通过反射调用方法
	returnValues := f.Call([]reflect.Value{s.rcvr, argv, replyv})
	if errInter := returnValues[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}

// 参数是任意需要映射为服务的结构体实例
// 因为服务和方法是不一样的
func newService(revr interface{}) *service {
	//新初始化了一个service
	s := new(service)
	//传入参数进行赋值
	s.rcvr = reflect.ValueOf(revr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(revr)
	//这个函数是干什么的
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is not a valid service name", s.name)
	}
	//服务中的方法注册
	s.registerMethods()
	//返回此服务名
	return s
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	//fmt.Println(s.typ.Method(0))
	//fmt.Println(s.typ.Method(0).Type)
	//fmt.Println(s.typ.Method(0).Name)
	//fmt.Println(s.typ.Method(0).Func)
	//fmt.Println(s.typ.Method(0).Index)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		fmt.Println(method)
		mType := method.Type
		//检查参数是否符合三个传入，一个传出
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		//返回值有且只有 1 个，类型为 error
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		argType, replyType := mType.In(1), mType.In(2)
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		//过滤出符合条件的方法
		s.method[method.Name] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, method.Name)
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

// 将服务方法首先注册到server端
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	//服务名字放入到map中去
	_, dup := server.serviceMap.LoadOrStore(s.name, s)
	if dup {
		return errors.New("rpc: service already defined: " + s.name)
	}
	return nil
}

func Register(rcvr interface{}) error {
	return DefaultServer.Register(rcvr)
}

// 从map中发现服务名，进而找到方法，传入参数service.method
func (s *Server) findService(serviceMethod string) (svc *service, mtype *methodType, err error) {
	//分割参数
	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	//得到服务名和方法名
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]
	//从map中首先寻找
	svci, ok := s.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	//类型断言转化
	svc = svci.(*service)
	//从服务中寻找方法
	mtype = svc.method[methodName]
	if mtype == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return
}
