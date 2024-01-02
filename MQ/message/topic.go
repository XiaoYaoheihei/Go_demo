package message

import (
	"log"
	"mq/util"
)

//topic的作用是接受所有客户端的消息，让后将消息发送给所有绑定的channel

type Topic struct {
	name                string              //名称
	newChannelChan      chan util.ChanReq   //新增的channel管道
	channelMap          map[string]*Channel //维护的channel集合
	comingMsgChan       chan *Message       //接受消息的channel
	msgBufferChan       chan *Message       //消息的缓冲管道
	readSyncChan        chan struct{}       //和route部分配合
	routerSyncChan      chan struct{}       //和read部分配合使用保证map安全
	exitChan            chan util.ChanReq   //接受退出信号的管道
	channelWriteStarted bool                //是否已向 channel 发送消息
}

// 全局的topicMap和channel，订阅的时候生成新的topic
var (
	TopicMap     = make(map[string]*Topic)
	newTopicChan = make(chan util.ChanReq)
)

func NewTopic(name string, size int) *Topic {
	topic := &Topic{
		name:           name,
		newChannelChan: make(chan util.ChanReq),
		channelMap:     make(map[string]*Channel),
		comingMsgChan:  make(chan *Message),
		msgBufferChan:  make(chan *Message, size),
		readSyncChan:   make(chan struct{}),
		routerSyncChan: make(chan struct{}),
		exitChan:       make(chan util.ChanReq),
	}
	go topic.Router(size)
	return topic
}

func GetTopic(name string) *Topic {
	topicChan := make(chan interface{})
	newTopicChan <- util.ChanReq{
		Variable: name,
		Retchan:  topicChan,
	}
	//从topicChan中获取对应的topic
	return (<-topicChan).(*Topic)
}

func TopicFactory(size int) {
	var (
		topicReq util.ChanReq
		name     string
		topic    *Topic
		ok       bool
	)
	for {
		topicReq = <-newTopicChan
		name = topicReq.Variable.(string)
		//先从全局的map中查找
		if topic, ok = TopicMap[name]; !ok {
			//没有找到再进行创建
			topic = NewTopic(name, size)
			TopicMap[name] = topic
			log.Printf("TOPIC %s CREATED", name)
		}
		topicReq.Retchan <- topic
	}
}

//以上三个函数是Topic的工厂

// 维护channel
func (t *Topic) GetChannel(name string) *Channel {
	channelRet := make(chan interface{})
	t.newChannelChan <- util.ChanReq{
		Variable: name,
		Retchan:  channelRet,
	}
	return (<-channelRet).(*Channel)
}

// 推送消息给channel
func (t *Topic) PutMessage(msg *Message) {
	t.comingMsgChan <- msg
}

func (t *Topic) MessagePump(closeChan <-chan struct{}) {
	var msg *Message
	for {
		select {
		case msg = <-t.msgBufferChan:
		case <-closeChan:
			return
		}

		t.readSyncChan <- struct{}{}
		//保证map的并发安全
		for _, channel := range t.channelMap {
			go func(ch *Channel) {
				ch.PutMessage(msg)
			}(channel)
		}

		t.routerSyncChan <- struct{}{}
	}

}

// topic的事件处理
func (t *Topic) Router(size int) {
	var (
		msg       *Message
		closeChan = make(chan struct{})
	)
	for {
		select {
		//Topic获取对应的channel
		case chanReq := <-t.newChannelChan:
			channelName := chanReq.Variable.(string)
			channel, ok := t.channelMap[channelName]
			if !ok {
				//map中没有维护的对应的channel，需要新创建
				channel = NewChannel(channelName, size)
				t.channelMap[channelName] = channel
				log.Printf("TOPIC(%s): new channel(%s)", t.name, channel.name)
			}
			chanReq.Retchan <- channel
			if !t.channelWriteStarted {
				go t.MessagePump(closeChan)
				t.channelWriteStarted = true
			}
		//Topic接收到消息,写入到相应的channel中
		case msg = <-t.comingMsgChan:
			select {
			case t.msgBufferChan <- msg:
				log.Printf("TOPIC(%s) wrote message", t.name)
			default:
			}
		case <-t.readSyncChan:
			<-t.routerSyncChan
		//关闭Topic的信号
		case closeReq := <-t.exitChan:
			log.Printf("TOPIC(%s): closing", t.name)
			//首先关闭Topic关联的所有的channel
			for _, channel := range t.channelMap {
				err := channel.Close()
				if err != nil {
					log.Printf("ERROR: channel(%s) close - %s", channel.name, err.Error())
				}
			}
			close(closeChan)
			closeReq.Retchan <- nil
		}
	}
}

func (t *Topic) Close() error {
	errChan := make(chan interface{})
	t.exitChan <- util.ChanReq{
		Retchan: errChan,
	}

	err, _ := (<-errChan).(error)
	return err
}
