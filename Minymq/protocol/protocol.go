package protocol

import (
	"Minymq/message"
	"Minymq/utils"
	"bufio"
	"log"
	"strings"
)

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
func (p *Protocol) Execute(client StatefulReadWriter, params []string) ([]byte, error) {

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
	log.Printf("protocol: writing msg (%s) to client - %s", uuidStr, client.String(), string(msg.Context()))
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
