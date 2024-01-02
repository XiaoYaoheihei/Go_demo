package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	pb "helloworld/helloworld"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type service_method struct {
	ServiceName string
	MethodName  string
	//匿名函数字段
}

type components struct {
	content map[string]*service_method
}

type server struct {
	pb.UnimplementedFirstServer
	//使用map存储组件名字，组件里面包含有对应的service和method
	cmps components
}

func (s *server) SayHello(ctx context.Context, in *pb.Request) (*pb.Response, error) {
	return &pb.Response{Message: "hello " + in.GetName()}, nil
}
func Newserver() *server {
	s := &server{}
	return s
}
func main() {
	flag.Parse()
	lis, err := net.Listen("tcp", "127.0.0.1:9009")
	if err != nil {
		log.Println(err)
		log.Fatal("fail to listen")
	}
	//读取配置文件的信息，将服务绑定到当前的server上
	//通过读取配置文件将json字符串反序列化成对应的字段
	test()
	grpcserver := grpc.NewServer()
	pb.RegisterFirstServer(grpcserver, Newserver())
	reflection.Register(grpcserver)
	grpcserver.Serve(lis)
}

func test() {
	str := "{\"ServiceName\":\"first\",\"MethodName\":\"first2\"}"
	m := service_method{}
	err := json.Unmarshal([]byte(str), &m)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(m.ServiceName, m.MethodName)
}
