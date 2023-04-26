package geecache

// 他的方法用于根据传入的key选择相应的http客户端(PeerGetter)
type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// 抽象出来的http客户端,他的Get方法用于从对应的group中查找缓存值
type PeerGetter interface {
	Get(group string, key string) ([]byte, error)
}
