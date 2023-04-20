// topics包含着多个topic
// topic的起到一个中转的作用，接收到Msg之后，将Msg同时发送给他绑定的所有channel上
package message

import (
	"Minymq/utils"
	"errors"
	"log"
)

type Topic struct {
	name string
	//新增channel的管道
	newChannelChan chan utils.ChanReq
	//维护的chaanel集合
	channelMap map[string]*Chan
	//接收消息的管道
	inComingMsgChan chan *message
	//缓冲管道，相当于消息的内存队列
	bufChan chan *message
	//和routerSyncChan配合使用保证channelMap的并发安全
	readSyncChan   chan struct{}
	routerSyncChan chan struct{}
	//接收退出信号的管道
	exitChan chan utils.ChanReq
	//是否已经向channel发送消息
	channelWriteStarted bool
}

// 全局变量用来存储一些信息
var (
	TopicMap     = make(map[string]*Topic)
	newTopicChan = make(chan utils.ChanReq)
)

// 工厂模式生成一个Topic
func NewTopic(name string, inMemSize int) *Topic {
	topic := &Topic{
		name:            name,
		newChannelChan:  make(chan utils.ChanReq),
		channelMap:      make(map[string]*Chan),
		inComingMsgChan: make(chan *message),
		bufChan:         make(chan *message, inMemSize),
		readSyncChan:    make(chan struct{}),
		routerSyncChan:  make(chan struct{}),
		exitChan:        make(chan utils.ChanReq),
	}
	//开启Topic之后进行事件循环
	go topic.Router(inMemSize)
	return topic
}

// close当前Topic
func (t *Topic) Close() error {
	t.exitChan <- utils.ChanReq{
		Variable: true,
	}
	return errors.New(t.name + "is ready to close ")
}

// 获取新的Topic
func GetTopic(name string) *Topic {

	newTopicChan <- utils.ChanReq{Variable: name}

}

// 获取新的channel
func (t *Topic) GetChannel(channelName string) *Chan {

	t.newChannelChan <- utils.ChanReq{
		Variable: channelName,
	}

}

// 我们需要维护一个全局的 topic map，在消费者订阅时生成新的 topic，类似于一个工厂
func TopicFactory(inMemSize int) {
	var (
		topicReq utils.ChanReq
		name     string
		topic    *Topic
		ok       bool
	)
	for {
		//有新的Topic
		topicReq = <-newTopicChan
		name = topicReq.Variable.(string)
		//检查此时的Map中是否含有此Topic
		if topic, ok = TopicMap[name]; !ok {
			topic = NewTopic(name, inMemSize)
			//全局的TopicMap来存储信息
			TopicMap[name] = topic
			log.Printf("topic (%s) is created", name)
		}
	}
}

// 推送消息给channel
func (t *Topic) PutMsg(msg *message) {
	t.inComingMsgChan <- msg
}
func (t *Topic) MsgPump(closeChan <-chan struct{}) {
	var msg *message
	for {
		select {
		case msg = <-t.bufChan:

		case <-closeChan:
			return
		}
		//配合使用readSyncChan和routerSyncChan来实现并发向channel中写
		t.readSyncChan <- struct{}{}

		for _, channel := range t.channelMap {
			go func(ch *Chan) {
				ch.PutMessage(msg)
			}(channel)
		}

		t.routerSyncChan <- struct{}{}
	}
}

// Topic下的事件循环
func (t *Topic) Router(inMemSize int) {
	var (
		msg       *message
		closeChan chan struct{}
	)
	for {
		select {
		//有需要新增channel
		case channelReq := <-t.newChannelChan:
			channelName := channelReq.Variable.(string)
			//channelMap中维护的是最基本的chan，最基本的chan下面多个channel
			channel, ok := t.channelMap[channelName]
			if !ok {
				channel = NewChan(channelName, inMemSize)
				t.channelMap[channelName] = channel
				log.Printf("topic(%s) : new channel (%s) ", t.name, channel.name)
			}
			//？？？？目前还不太清楚这里是干什么的
			if !t.channelWriteStarted {
				go t.MsgPump(closeChan)
				t.channelWriteStarted = true
			}
		//有消息新增
		case msg = <-t.inComingMsgChan:
			select {
			case t.bufChan <- msg:
				log.Printf("tpoic(%s) wrote message ", t.name)
			default:

			}
		//并发处理的关键点
		case <-t.readSyncChan:
			<-t.routerSyncChan
		//有关闭当前Topic下所有channel的需求
		case <-t.exitChan:
			log.Printf("topic (%s) is closing ", t.name)
			for _, channel := range t.channelMap {
				err := channel.Close()
				if err != nil {
					log.Printf("error : channel (%s) close --(%s)", channel.name, error.Error())
				}
			}
			close(closeChan)
		}

	}
}
