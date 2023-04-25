package geecache

import (
	"geecache/lru"
	"sync"
)

type cache struct {
	//互斥锁
	mutex      sync.Mutex
	lru        *lru.Cache
	cacheBytes int64
}

// 外层封装了Add方法
func (c cache) add(key string, value ByteView) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.lru == nil {
		c.lru = lru.New(c.cacheBytes, nil)
	}
	c.lru.Add(key, value)
	return nil
}

// 外层封装了Get方法
func (c cache) get(key string) (value ByteView, ok bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.lru == nil {
		return ByteView{}, false
	}

	if v, ok := c.lru.Get(key); ok {
		return v.(ByteView), ok
	}

	return ByteView{}, false
}
