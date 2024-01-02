package message

// 消息就是普通的字节数组
type Message struct {
	//前16位是uid，作为唯一标识
	//后面的就是消息本身的内容
	data     []byte
	timeChan chan struct{} //取消某一条消息的等待超时
}

func NewMessage(data []byte) *Message {
	return &Message{
		data: data,
	}
}

func (m *Message) Getuid() []byte {
	return m.data[:16]
}

func (m *Message) Getbody() []byte {
	return m.data[16:]
}

func (m *Message) Getdata() []byte {
	return m.data
}

// 如果该消息已经确认了，终止该消息超时等待的goroutine
func (m *Message) Endtimer() {
	select {
	case m.timeChan <- struct{}{}:
	default:

	}
}
