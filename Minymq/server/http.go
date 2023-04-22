package server

// HTTP服务器
import (
	"Minymq/message"
	"Minymq/protocol"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// 重新拿到需要的请求头和请求体
type ReqParams struct {
	params url.Values
	body   []byte
}

func NewReqParams(req *http.Request) (*ReqParams, error) {
	reqParams, err := url.ParseQuery(req.URL.RawQuery)
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	return &ReqParams{reqParams, data}, nil
}

// 启动一个httpServer
func HttpServer(ctx context.Context, address string, port string, endChan chan struct{}) {
	http.HandleFunc("/ping", pingHandler)
	http.HandleFunc("/put", putHandler)
	http.HandleFunc("/stats", statHandler)

	fqAddress := address + ":" + port
	httpServer := http.Server{
		Addr: fqAddress,
	}

	go func() {
		log.Printf("listening for http requests on %s", fqAddress)
		err := http.ListenAndServe(fqAddress, nil)
		if err != nil {
			log.Fatal("http.ListenAndServe:", err)
		}
	}()

	<-ctx.Done()
	log.Printf("HTTP server on %s is shutdowning...", fqAddress)
	timeoutCtx, fn := context.WithTimeout(context.Background(), 10*time.Second)
	defer fn()
	if err := httpServer.Shutdown(timeoutCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}
	close(endChan)
}

// 查询相关信息
func (r *ReqParams) Query(key string) (string, error) {
	keyData := r.params[key]
	if len(keyData) == 0 {
		return "", errors.New("key not in query params")
	}
	return keyData[0], nil
}

// 测试连接
func pingHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("content-length", "2")
	io.WriteString(w, "ok")
}

// 写入消息
func putHandler(w http.ResponseWriter, req *http.Request) {
	reqParams, err := NewReqParams(req)
	if err != nil {
		log.Printf("http : error - %s", err.Error())
		return
	}

	topicName, err := reqParams.Query("topic")
	if err != nil {
		log.Printf("http: error - %s", err.Error())
		return
	}

	//包装了一个假的客户端向协议中发送PUB指令，由协议与Topic进行交互
	conn := &FakeConn{}
	broker := NewBroker(conn, "http")
	proto := &protocol.Protocol{}
	resp, err := proto.Execute(broker, "PUB", topicName, string(reqParams.body))
	if err != nil {
		log.Printf("http: error - %s", err.Error())
		return
	}

	w.Header().Set("Content-Length", strconv.Itoa(len(resp)))
	w.Write(resp)
}

// 查看所有的topic
func statHandler(w http.ResponseWriter, req *http.Request) {
	for topicName := range message.TopicMap {
		io.WriteString(w, fmt.Sprintf("%s\n", topicName))
	}
}
