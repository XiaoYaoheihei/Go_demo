// 抽象了一个"只读"数据结构 ByteView 用来表示缓存值
// 是 GeeCache 主要的数据结构之一。

package geecache

type ByteView struct {
	// b 将会存储真实的缓存值,
	//选择 byte 类型是为了能够支持任意的数据类型的存储
	b []byte
}

func (tmp ByteView) Len() int {
	return len(tmp.b)
}

// 因为b是只读的，所以我们使用此方法返回一个拷贝值，防止缓存值被修改
func (tmp ByteView) ByteSlice() []byte {
	return cloneBytes(tmp.b)
}

// 返回一个string类型值
func (tmp ByteView) String() string {
	return string(tmp.b)
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
