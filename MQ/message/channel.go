package message

import (
	"errors"
	"log"
	"mq/util"
	"time"
)

type Consumer interface {
	Close()
}

type Channel struct {
	name         string
	addClient    chan util.ChanReq //增加消费的信息
	removeClient chan util.ChanReq //减少消费者的信息
	clients      []Consumer        //消费者数组

	produceMessgaeChan  chan *Message //接受producer信息的管道
	bufferChann         chan *Message //缓冲message的管道
	consumerMessageChan chan *Message //消息会被发送到此管道，后续会由消费者拉取

	exitChan chan util.ChanReq //关闭信号

	flightMessageChan chan *Message       //已经发送的消息管道
	flightMessages    map[string]*Message //已发送的消息map

	finishMessageChan chan util.ChanReq //消息确认

	requeueMessageChan chan util.ChanReq //消息重入队列
}

func NewChannel(name string, size int) *Channel {
	channel := &Channel{
		name:                name,
		addClient:           make(chan util.ChanReq),
		removeClient:        make(chan util.ChanReq),
		clients:             make([]Consumer, 0, 5),
		produceMessgaeChan:  make(chan *Message, 5),
		bufferChann:         make(chan *Message, size),
		consumerMessageChan: make(chan *Message),
		exitChan:            make(chan util.ChanReq),
		flightMessageChan:   make(chan *Message),
		flightMessages:      make(map[string]*Message),
		requeueMessageChan:  make(chan util.ChanReq),
		finishMessageChan:   make(chan util.ChanReq),
	}

	go channel.Route()
	return channel

}

func (c *Channel) AddClient(client Consumer) {
	log.Printf("Channel(%s): adding client...", c.name)
	//用来同步的channel
	done := make(chan interface{})
	c.addClient <- util.ChanReq{
		Variable: client,
		Retchan:  done,
	}
	//阻塞等待
	<-done
}

func (c *Channel) RemoveClient(client Consumer) {
	log.Printf("Channel(%s): removing client...", c.name)
	done := make(chan interface{})
	c.removeClient <- util.ChanReq{
		Variable: client,
		Retchan:  done,
	}
	//阻塞等待
	<-done
}

// 后台异步线程处理事件
func (c *Channel) Route() {
	var clientReq util.ChanReq
	closeChan := make(chan struct{})

	go c.MessagePump(closeChan)
	go c.RequeueRouter(closeChan)

	for {
		select {
		//检查增删消费者信息
		case clientReq = <-c.addClient:
			client := clientReq.Variable.(Consumer)
			c.clients = append(c.clients, client)
			log.Printf("CHANNEL(%s) added client %#v", c.name, client)
			//同步消息
			clientReq.Retchan <- struct{}{}
		case clientReq = <-c.removeClient:
			client := clientReq.Variable.(Consumer)
			index := -1
			//查找对应的consumer
			for number, v := range c.clients {
				if v == client {
					index = number
					break
				}
			}

			if index == -1 {
				log.Printf("ERROR: could not find client(%#v) in clients(%#v)", client, c.clients)
			} else {
				//删除对应的client之后重新整理clients
				c.clients = append(c.clients[:index], c.clients[index+1:]...)
				log.Printf("CHANNEL(%s) removed client %#v", c.name, client)
			}
			clientReq.Retchan <- struct{}{}

		//检查是否生产消息
		case msg := <-c.produceMessgaeChan:
			//当生产者生产了消息之后，将其放入到bufferChan中
			//bufferChan中数据填满之后造成阻塞的话直接default丢弃消息
			select {
			case c.bufferChann <- msg:
				log.Printf("CHANNEL(%s) wrote message", c.name)
			default:
			}

			//检查是否有退出消息
		case closeReq := <-c.exitChan:
			//关闭此channel,所有关联closeChan的goroutine都会收到信息
			close(closeChan)

			for _, consumer := range c.clients {
				consumer.Close()
			}

			closeReq.Retchan <- nil
		}
	}
}

func (c *Channel) PutMessage(msg *Message) {
	c.produceMessgaeChan <- msg
}

func (c *Channel) PullMessage() *Message {
	return <-c.consumerMessageChan
}

// message进行转移
func (c *Channel) MessagePump(close chan struct{}) {
	var msg *Message
	for {
		select {
		//bufferChan中如果没有数据的话在这里阻塞
		case msg = <-c.bufferChann:
		case <-close:
			//有关闭信号的话直接结束此goroutine
			return
		}
		if msg != nil {
			c.flightMessageChan <- msg
		}
		//将缓冲的消息直接发送给consumer拉取的channel
		c.consumerMessageChan <- msg
	}
}

// 消息重入
func (c *Channel) RequeueMessage(uuid string) error {
	errorChan := make(chan interface{})
	c.requeueMessageChan <- util.ChanReq{
		Variable: uuid,
		Retchan:  errorChan,
	}

	err, _ := (<-errorChan).(error)
	return err
}

func (c *Channel) RequeueRouter(close chan struct{}) {
	for {
		select {
		//将已经发送的消息记录下来
		case msg := <-c.flightMessageChan:
			c.pushInMap(msg)
			go func(msg *Message) {
				//消费者如果迟迟不确认此消息的话，消息就会堆积，map中就会存在很多数据
				//做法是定时处理，如果在限定时间内没有完成确认，会自动将该消息重新入队
				select {
				case <-time.After(60 * time.Second):
					log.Printf("CHANNEL(%s): auto requeue of message(%s)", c.name, util.UuidTostring(msg.Getuid()))
				case <-msg.timeChan:
					//此消息已经被确认了，不会执行此定时任务，不会将消息重入
					return
				}
				err := c.RequeueMessage(util.UuidTostring(msg.Getuid()))
				if err != nil {
					log.Printf("ERROR: channel(%s) - %s", c.name, err.Error())
				}
			}(msg)
		//收到了确认消息的通知
		case finishReq := <-c.finishMessageChan:
			uuid := finishReq.Variable.(string)
			_, err := c.popInMap(uuid)
			if err != nil {
				log.Printf("ERROR: failed to finish message(%s) - %s", uuid, err.Error())
			}
			finishReq.Retchan <- err
		//消息重入的通知
		case requeueReq := <-c.requeueMessageChan:
			uuid := requeueReq.Variable.(string)
			msg, err := c.popInMap(uuid)
			if err != nil {
				log.Printf("ERROR: failed to requeue message(%s) - %s", uuid, err.Error())
			} else {
				go func(msg *Message) {
					//重新进行一次消息生产
					c.PutMessage(msg)
				}(msg)
			}
			requeueReq.Retchan <- err
		case <-close:
			return
		}
	}
}

func (c *Channel) Close() error {
	errChan := make(chan interface{})
	c.exitChan <- util.ChanReq{
		Retchan: errChan,
	}

	err, _ := (<-errChan).(error)
	return err
}

func (c *Channel) pushInMap(msg *Message) {
	c.flightMessages[util.UuidTostring(msg.Getuid())] = msg
}

func (c *Channel) popInMap(uuid string) (*Message, error) {
	msg, ok := c.flightMessages[uuid]
	if !ok {
		return nil, errors.New("UUID not in flight")
	}
	//在记录中删除消息相关
	delete(c.flightMessages, uuid)
	msg.Endtimer()
	return msg, nil
}

// 消息确认的相关逻辑
func (c *Channel) FinishMessage(uuid string) error {
	errChan := make(chan interface{})
	c.finishMessageChan <- util.ChanReq{
		Variable: uuid,
		Retchan:  errChan,
	}
	//同步等待
	err, _ := (<-errChan).(error)
	return err
}
