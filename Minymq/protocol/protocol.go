package protocol

import (
	"Minymq/message"
	"Minymq/utils"
	"bufio"
	"bytes"
	"log"
	"reflect"
	"strings"
)

// 协议关联了Channel,实现了“拉”模式
type Protocol struct {
	channel *message.Chan
}

// 从客户端读取输入并且简单过滤，拆分成各个参数传递给Execute方法
func (p *Protocol) IoLoop(client StatefulReadWriter) error {
	var (
		err  error
		line string
		resp []byte
	)
	//初始化状态
	client.SetState(ClientInit)
	//获取*Reader，从io读取的缓冲区对象
	reader := bufio.NewReader(client)
	for {
		//解析参数
		line, err = reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.Replace(line, "\n", "", -1)
		line = strings.Replace(line, "\r", "", -1)
		params := strings.Split(line, " ")
		log.Printf("protocol : %v", params)
		//执行参数
		resp, err = p.Execute(client, params)
		//检查返回值情况，搞清楚break和continue的意义
		if err != nil {
			_, err = client.Write([]byte(err.Error()))
			if err != nil {
				break
			}
			continue
		}
		if resp != nil {
			_, err = client.Write(resp)
			if err != nil {
				break
			}
		}
	}
	return err
}

// 执行参数，以传入的 params 的第一项作为方法名，判断有无实现该函数并执行反射调用。
func (p *Protocol) Execute(client StatefulReadWriter, params ...string) ([]byte, error) {
	var (
		err  error
		resp []byte
	)
	//获取Type类型
	typ := reflect.TypeOf(p)
	//声明一个Value类型的切片
	args := make([]reflect.Value, 3)
	args[0] = reflect.ValueOf(p)
	args[1] = reflect.ValueOf(client)
	//返回将所有字母都转为对应的大写版本的拷贝
	cmd := strings.ToUpper(params[0])
	//通过Type类型来判断是否存在某个方法
	if method, ok := typ.MethodByName(cmd); ok {
		args[2] = reflect.ValueOf(params)
		//存在方法就进行调用，传入参数是一个切片[]Value，返回值也是个切片[]Value
		//将 client 和 params 数组一起作为参数进行传递
		returnValues := method.Func.Call(args)
		//它返回函数所有输出结果的Value封装的切片
		if !returnValues[0].IsNil() {
			resp = returnValues[0].Interface().([]byte)
		}
		if !returnValues[1].IsNil() {
			err = returnValues[1].Interface().(error)
		}
		return resp, err
	}
	return nil, ClientErrInvalid
}

// 执行SUB订阅
func (p *Protocol) SUB(client StatefulReadWriter, params []string) ([]byte, error) {
	if client.GetState() != ClientInit {
		return nil, ClientErrInvalid
	}
	//参数有问题
	if len(params) < 3 {
		return nil, ClientErrInvalid
	}
	topicName := params[1]
	if len(topicName) == 0 {
		return nil, ClientErrBadTopic
	}
	channelName := params[2]
	if len(channelName) == 0 {
		return nil, ClientErrBadChannel
	}
	client.SetState(ClientWaitGet)
	//获取topic和channel
	topic := message.GetTopic(topicName)
	//绑定协议与channel之间的关联
	p.channel = topic.GetChannel(channelName)
	return nil, nil
}

// 执行GET读取
func (p *Protocol) GET(client StatefulReadWriter, params []string) ([]byte, error) {
	if client.GetState() != ClientWaitGet {
		return nil, ClientErrInvalid
	}
	//首先拉取msg
	msg := p.channel.PullMessage()
	if msg == nil {
		log.Printf("error : msg == nil")
		return nil, ClientErrBadMessage
	}
	//得到uuid
	uuidStr := utils.UuidTostring(msg.Uuid())
	log.Printf("protocol: writing msg (%s) to client(%s) - %s", uuidStr, client.String(), string(msg.Context()))
	//更改状态
	client.SetState(ClientWaitResponse)
	return msg.Data(), nil
}

// 执行FIN完成
func (p *Protocol) FIN(client StatefulReadWriter, params []string) ([]byte, error) {
	if client.GetState() != ClientWaitResponse {
		return nil, ClientErrInvalid
	}
	if len(params) < 2 {
		return nil, ClientErrInvalid
	}
	//获取uuid
	uuidStr := params[1]
	//获取是否将消息已经确认
	err := p.channel.FinMessage(uuidStr)
	if err != nil {
		return nil, err
	}
	//更改状态
	client.SetState(ClientWaitGet)

	return nil, nil
}

// 执行REQ重入
func (p *Protocol) REQ(client StatefulReadWriter, params []string) ([]byte, error) {
	if client.GetState() != ClientWaitResponse {
		return nil, ClientErrInvalid
	}
	if len(params) < 2 {
		return nil, ClientErrInvalid
	}

	uuidStr := params[1]
	//msg重新入队列是否成功
	err := p.channel.RepeatMessage(uuidStr)
	if err != nil {
		return nil, err
	}
	client.SetState(ClientWaitGet)

	return nil, nil
}

func (p *Protocol) PUB(client StatefulReadWriter, params []string) ([]byte, error) {
	var buf bytes.Buffer
	var err error
	if client.GetState() != -1 {
		return nil, ClientErrInvalid
	}
	if len(params) < 3 {
		return nil, ClientErrInvalid
	}

	topicName := params[1]
	body := []byte(params[2])
	//拿到消息的uuid
	_, err = buf.Write(<-utils.UuidChan)
	if err != nil {
		return nil, err
	}
	//需要发送的消息放入缓冲区中
	_, err = buf.Write(body)
	if err != nil {
		return nil, err
	}

	topic := message.GetTopic(topicName)
	topic.PutMsg(message.NewMessage(buf.Bytes()))
	return []byte("ok"), nil
}
