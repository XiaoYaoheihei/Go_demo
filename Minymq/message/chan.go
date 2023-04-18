// chan这个组件，里面存放着message实体，还存放着其他信息
package message

import (
	"Minymq/utils"
	"errors"
	"log"
	"time"
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
	//重新需要确认的消息管道
	inFlightMsgChan chan *message
	//存储已经发送的消息
	inFlightMsg map[string]*message
	//已经Ack的消息Req
	finMsgChan chan utils.ChanReq
	//多次消费
	repreatMsgChan chan utils.ChanReq
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
	//需要返回一个错误信息
}

// 在map中存储消息
func (c *Chan) pushInFlightMsg(msg *message) {
	c.inFlightMsg[utils.UuidTostring(msg.Uuid())] = msg
}

// 从map中删除消息
func (c *Chan) popInFlightMsg(uidStr string) (*message, error) {
	//查找消息
	msg, ok := c.inFlightMsg[uidStr]
	if !ok {
		return nil, errors.New("uuid not in FlightMsg")
	}
	//删除已经ack的消息
	delete(c.inFlightMsg, uidStr)
	//向管道中写
	msg.EndTime()
	return msg, nil
}

// 确认消息的相关逻辑
func (c *Chan) FinMessage(uuidStr string) error {
	c.finMsgChan <- utils.ChanReq{
		Variable: uuidStr,
	}
	return errors.New(uuidStr + "has Acked!")
}

// 多次消费某一条消息
func (c *Chan) RepeatMessage(uuidStr string) error {
	c.repreatMsgChan <- utils.ChanReq{
		Variable: uuidStr,
	}
	return errors.New(uuidStr + " start to repeat")
}

// 后台事件循环
func (c *Chan) EventLoop() {
	var (
		conReq    utils.ChanReq
		closeChan = make(chan struct{})
	)
	//将消息send到消费者管道中
	go c.MessagePump(closeChan)
	go c.ReAckQueue(closeChan)
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

// 需要Ack的所有消息
func (c *Chan) ReAckQueue(closeChan chan struct{}) {
	for {
		select {
		//检测FlightMsg管道中有无数据
		case msg := <-c.inFlightMsgChan:
			c.pushInFlightMsg(msg)
			//如果过多的消息积压，需要释放或者是相同的消息进行合并（我的想法）
			//在限定的时间内如果消息没有确认完成的话，我们就将该消息自动重新入队。
			go func(msg *message) {
				select {
				case <-time.After(60 * time.Second):
					log.Printf("")
				//msg确认完成之后需要终止这个等待超时的逻辑,直接退出此协程就好
				case <-msg.timer:
					return
				}
				//超时之后重新将消息入队，先在inFlight中删除，然后按照repeat的逻辑
				//真的有必要重新放入到生产者对应的队列中吗？？？？
				err := c.RepeatMessage(utils.UuidTostring(msg.Uuid()))
				if err != nil {
					log.Printf("error : channel(%s) - (%s)", c.name, err.Error())
				}
			}(msg)
		//检测已经Ack的Req管道有无数据
		case fin := <-c.finMsgChan:
			uuidStr := fin.Variable.(string)
			//删除对应FlightMsg中的数据
			_, err := c.popInFlightMsg(uuidStr)
			if err != nil {
				log.Printf("error : failed to finish msg(%s)", uuidStr, err.Error())
			}
		//重复消费某一条消息
		case repeatMsg := <-c.repreatMsgChan:
			uuidStr := repeatMsg.Variable.(string)
			msg, err := c.popInFlightMsg(uuidStr)
			if err != nil {
				log.Printf("error : failed to repeat consumer message (%s)", uuidStr, error.Error())
			} else {
				//重新放入生产者消息队列
				go func(msg *message) {
					c.PutMessage(msg)
				}(msg)
			}
		case <-closeChan:
			log.Printf("ReAckQueue has closed")
			return
		}
	}
}

// MessagePump会将buf中的消息发送至消费者管道
func (c *Chan) MessagePump(closeChan chan struct{}) {
	var msg *message
	for {
		select {
		case msg = <-c.bufChan:
			log.Printf("conMsgChan has get %s", msg)
		case <-closeChan:
			log.Printf("MessagePump has closed")
			return
		}
		//如果消息不为空的话,放入待确认的FlightMsg中
		if msg != nil {
			c.inFlightMsgChan <- msg
		}
		c.conMsgChan <- msg
	}
}
