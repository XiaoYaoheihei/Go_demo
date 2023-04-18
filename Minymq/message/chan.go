// chan这个组件，里面存放着message实体，还存放着其他信息
package message

import (
	"Minymq/utils"
	"log"
)

type Consumer interface {
	Close()
}

type Chan struct {
	name string
	//添加consumer管道
	addChan chan utils.ChanReq
	//删除consumer管道
	remChan chan utils.ChanReq
	//绑定consumer
	consumers []Consumer
	//缓冲管道，用来暂存消息
	bufChan chan *message
	//生产者管道，存放生产者消息
	proMsgChan chan *message
	//消费者管道，存放消费者消息,消费者后续从这里拉取消息
	conMsgChan chan *message
	//关闭管道
	exitChan chan utils.ChanReq
}

// 添加消费者
func (c *Chan) Add(conInfo Consumer) {
	log.Printf("Channel(%s)is adding consumer...", c.name)
	//将添加消费者的信息添加到管道中
	c.addChan <- utils.ChanReq{
		Variable: conInfo,
	}
}
func (c *Chan) Remove(conInfo Consumer) {
	log.Printf("Channel(%s)is removing consumer...", c.name)
	//将删除消费者的信息添加到管道中
	c.remChan <- utils.ChanReq{
		Variable: conInfo,
	}
}

// 后台事件循环
func (c *Chan) EventLoop() {
	var (
		conReq    utils.ChanReq
		closeChan = make(chan struct{})
	)

	go c.MessagePump(closeChan)
	for {
		select {
		//有添加消费者的消息到来
		case conReq = <-c.addChan:
			//类型断言
			consumer := conReq.Variable.(Consumer)
			c.consumers = append(c.consumers, consumer)
			log.Printf("Channel (%s) has added consumer%v", c.name, consumer)
		//有删除消费者的消息到来
		case conReq = <-c.remChan:
			consumer := conReq.Variable.(Consumer)
			indexRemove := -1
			//找到需要删除的下标
			for k, v := range c.consumers {
				if v == consumer {
					indexRemove = k
					break
				}
			}
			//对删除的下标进行判断
			if indexRemove == -1 {
				log.Printf("error : couldn't find consumer %v in consumers%v", consumer, c.consumers)
			} else {
				c.consumers = append(c.consumers[:indexRemove], c.consumers[indexRemove+1:]...)
				log.Printf("success : Channel remove consumer %v in consumers%s", consumer, c.consumers)
			}
		//生产者管道中有消息来
		case msg := <-c.proMsgChan:
			// 防止因 bufChan 缓冲填满时造成阻塞，加上一个 default 分支直接丢弃消息
			select {
			case c.bufChan <- msg:
				log.Printf("channel %s had wroted message", c.name)
			default:
				log.Printf("channel's(%s) buffchannel is full of message", c.name)
			}
		//当检测到关闭管道有消息来的时候，需要释放channel的所有资源
		case closeReq := <-c.exitChan:
			log.Printf("channel %s is ready to close", c.name)
			close(closeChan)
			for _, consumer := range c.consumers {
				consumer.Close()
			}
			closeReq.Variable = nil
		}
	}
}

// 生产者发送来消息
func (c *Chan) PutMessage(msg *message) {
	c.proMsgChan <- msg
}

// 消费者消费消息
func (c *Chan) PullMessage() *message {
	return <-c.conMsgChan
}

// 关闭chan，释放chan包含的所有资源
func (c *Chan) Close() error {
	//首先向exitChan中传入一个bool值
	c.exitChan <- utils.ChanReq{
		Variable: true,
	}

}

// MessagePump会将buf中的消息发送至消费者管道
func (c *Chan) MessagePump(closeChan chan struct{}) {
	var msg *message
	for {
		select {
		case msg := <-c.bufChan:
			log.Printf("conMsgChan has get %s", msg)
		case <-closeChan:
			log.Printf("MessagePump has closed")
			return
		}
		c.conMsgChan <- msg
	}
}
