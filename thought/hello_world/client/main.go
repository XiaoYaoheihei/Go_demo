package main

import (
	"flag"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "helloworld/helloworld"
	"log"
	"time"
)

func main() {
	flag.Parse()
	//var opts []grpc.DialOption

	conn, err := grpc.Dial("127.0.0.1:9009", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Println(err)
		log.Fatal("fail to connect")
	}
	defer conn.Close()

	client := pb.NewFirstClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	feature, err := client.SayHello(ctx, &pb.Request{Name: "yao mou"})
	if err != nil {
		log.Fatal("get feature failed")
	}
	log.Println(feature)
}
