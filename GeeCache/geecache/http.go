package geecache

import (
	"fmt"
	"geecache/consistenthash"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	defaultBasePath = "/_geecache/"
	defaultReplicas = 50
)

// 服务端类
type HTTPPool struct {
	//用来记录自己地址
	self string
	//作为节点间通讯地址的前缀
	basePath string
	mutex    sync.Mutex
	//一致性哈希的map，用来根据具体的key选择节点
	peers *consistenthash.Map
	//映射远程节点与对应的 httpGetter
	//每一个远程节点对应一个 httpGetter，因为 httpGetter 与远程节点的地址 baseURL 有关
	httpGetters map[string]*httpGetter
}

// 客户端类
type httpGetter struct {
	//表示将要访问的远程节点的地址
	baseURL string
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", h.self, fmt.Sprintf(format, v...))
}

// 服务端的实现逻辑
func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//首先检查访问的路由是否有规定前缀
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	//日志打印出相应的信息
	p.Log("%s %s", r.Method, r.URL.Path)
	//对参数进行分割
	parts := strings.SplitN(r.URL.Path[len(h.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "had request ", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]
	//查找分组
	group := GetGroup(groupName)
	if group == nil {
		http.Error(w, "no such group:"+groupName, http.StatusBadRequest)
		return
	}
	//查找内容
	view, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(view.ByteSlice())
}

func (p *HTTPPool) Set(peers ...string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	//实例了一致性哈希算法
	p.peers = consistenthash.New(defaultReplicas, nil)
	//添加了传入的节点
	p.peers.Add(peers...)
	p.httpGetters = make(map[string]*httpGetter, len(peers))
	//为每一个节点创建了一个http客户端
	for _, peer := range peers {
		p.httpGetters[peer] = &httpGetter{
			baseURL: peer + p.basePath,
		}
	}
}

// 根据具体的key选择对应的节点
func (p *HTTPPool) PickPeer(key string) (PeerGetter, bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	peer := p.peers.Get(key)
	if peer != "" && peer != p.self {
		p.Log("pick peer %s", peer)
		//返回对应的分布式节点实例
		return p.httpGetters[peer], true
	}
	return nil, false
}

var _ PeerPicker = (*HTTPPool)(nil)

func (h *httpGetter) Get(group string, key string) ([]byte, error) {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(group),
		url.QueryEscape(key),
	)
	//通过http的通信方式访问远程节点的地址并且获取返回值
	res, err := http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned: %v", res.Status)
	}
	//读取Body部分的所有内容
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}
	return bytes, nil
}
