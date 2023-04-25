package geecache

import (
	"errors"
	"fmt"
	"log"
	"sync"
)

type Getter interface {
	//回调函数
	Get(key string) ([]byte, error)
}

type Group struct {
	name      string
	getter    Getter
	mainCache cache
}

// 函数类型GetterFunc
type GetterFunc func(key string) ([]byte, error)

// 接口型函数，函数类型实现了接口中的方法
func (g GetterFunc) Get(key string) ([]byte, error) {
	return g(key)
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// 一个Group可以认为是一个缓存的命名空间
func NewGroup(name string, bytes int64, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:   name,
		getter: getter,
		mainCache: cache{
			cacheBytes: bytes,
		},
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	//只读锁
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

func (g *Group) Get(key string) (ByteView, error) {
	//如果查找的key是空string
	if key == "" {
		return ByteView{}, fmt.Errorf("key is required")
	}

	//从 mainCache 中查找缓存，如果存在则返回缓存值。
	v, ok := g.mainCache.get(key)
	if ok {
		log.Println("[GeeCache] hit")
		return v, nil
	}
	fmt.Println(ok)
	//缓存不存在，则调用 load 方法
	//fmt.Println(key, " not find in cache")
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	return g.getLocally(key)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	//调用用户回调函数 g.getter.Get() 获取源数据
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	value := ByteView{b: cloneBytes(bytes)}
	fmt.Println(bytes, value)
	//将源数据添加到缓存 mainCache
	err = g.populateCache(key, value)
	if err != nil {
		fmt.Println("populate failed")
	}
	fmt.Println(err)
	return value, nil
}

// 填充到mainCache中去
func (g *Group) populateCache(key string, value ByteView) error {
	err := g.mainCache.add(key, value)
	if err != nil {
		fmt.Println("add failed")
		return errors.New("add failed")
	}
	return nil
}
