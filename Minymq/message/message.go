package message

// 定义消息实体
type message struct {
	//数据中有Uuid和具体内容
	data []byte
	//timer用来标记已经被ack，退出相应的逻辑
	timer chan bool
}

// 使用工厂模式返回一个消息实例
func NewMessage(data []byte) *message {
	return &message{
		data: data,
	}
}

// 返回Uuid,是消息的唯一标识符
func (m *message) Uuid() []byte {
	return m.data[:16]
}

// 返回消息中的具体内容
func (m *message) Context() []byte {
	return m.data[16:]
}

// 返回整个消息，包含uuid和context
func (m *message) Data() []byte {
	return m.data
}

// 消息已经被ack，向管道中填入true
func (m *message) EndTime() {
	select {
	case m.timer <- true:
	default:
	}
}
