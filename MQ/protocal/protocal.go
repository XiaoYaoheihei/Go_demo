package protocal

import (
	"bufio"
	"log"
	"mq/message"
	"reflect"
	"strings"
)

type Protocal struct {
	channel *message.Channel
}

// 循环从客户端读取输入，交给Execute处理
func (p *Protocal) IOloop(client StateReadWrite) error {
	var err error
	var line string
	var resp []byte
	client.SetState(ClientInit)

	reader := bufio.NewReader(client)
	for {
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		//替换string
		line = strings.Replace(line, "\n", "", -1)
		line = strings.Replace(line, "\r", "", -1)
		params := strings.Split(line, " ")
		log.Printf("PROTOCOL: %#v", params)

		//执行具体的流程
		resp, err = p.Execute(client, params...)
		if err != nil {

		}

		if resp != nil {

		}
	}
	return err
}

func (p *Protocal) Execute(client StateReadWrite, params ...string) ([]byte, error) {
	var err error
	var resp []byte
	//获取具体的类型信息
	typ := reflect.TypeOf(p)
	args := make([]reflect.Value, 3)
	args[0] = reflect.ValueOf(p)
	args[1] = reflect.ValueOf(client)

	cmd := strings.ToUpper(params[0])
	//首先检查此类型是否实现了对应的方法
	if method, ok := typ.MethodByName(cmd); ok {
		args[2] = reflect.ValueOf(params)
		//实现了对应的方法就进行调用
		values := method.Func.Call(args)
		if !values[0].IsNil() {

		}
		if !values[1].IsNil() {

		}
		return resp, err
	}
	return nil, Invalid
}

// 订阅
func (p *Protocal) SUB(client StateReadWrite, params []string) ([]byte, error) {

}

// 读取
func (p *Protocal) GET(client StateReadWrite, params []string) ([]byte, error) {

}

// 完成
func (p *Protocal) FIN(client StateReadWrite, params []string) ([]byte, error) {

}

// 重入
func (p *Protocal) REQ(client StateReadWrite, params []string) ([]byte, error) {

}
