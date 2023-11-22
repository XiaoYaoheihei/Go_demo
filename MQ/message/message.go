package message

// 消息就是普通的字节数组
type Message struct {
	//前16位是uid，作为唯一标识
	//后面的就是消息本身的内容
	data []byte
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
