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
	peers     PeerPicker
	//loader *singleflight.Group	//确保key对应的请求只被调用一次
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
	//缓存不存在，则调用 load 方法
	//fmt.Println(key, " not find in cache")
	return g.load(key)
}

func (g *Group) load(key string) (ByteView, error) {
	if g.peers != nil {
		//使用PickPeer方法选择节点，若非本机节点，则从远程获取
		if peer, ok := g.peers.PickPeer(key); ok {
			value, err := g.getFromPeer(peer, key)
			if err == nil {
				return value, nil
			}
			log.Println("[GeeCache] Failed to get from peer", err)
		}
	}
	//若是本机节点或者失败，回退至getLocally
	return g.getLocally(key)
}

func (g *Group) getLocally(key string) (ByteView, error) {
	//调用用户回调函数 g.getter.Get() 获取源数据
	bytes, err := g.getter.Get(key)
	if err != nil {
		return ByteView{}, err
	}

	value := ByteView{b: cloneBytes(bytes)}
	//fmt.Println(bytes, value)
	//将源数据添加到缓存 mainCache
	err = g.populateCache(key, value)
	if err != nil {
		fmt.Println("populate failed")
	}
	//fmt.Println(err)
	return value, nil
}

// 填充到mainCache中去
func (g *Group) populateCache(key string, value ByteView) error {
	err := g.mainCache.add(key, value)
	//fmt.Println("jianchashifou zhendde")
	//v, err1 := g.Get(key)
	if err != nil {
		fmt.Println("add failed")
		return errors.New("add failed")
	}
	return nil
}

// 将实现了 PeerPicker 接口的 HTTPPool 注入到 Group 中
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

// 使用实现了 PeerGetter 接口的 httpGetter 从访问远程节点，获取缓存值
func (g *Group) getFromPeer(peer PeerGetter, key string) (ByteView, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return ByteView{}, err
	}
	return ByteView{b: bytes}, nil
}
