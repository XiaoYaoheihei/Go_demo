package message

import (
	"log"
	"mq/util"
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
		//将缓冲的消息直接发送给consumer拉取的channel
		c.consumerMessageChan <- msg
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
