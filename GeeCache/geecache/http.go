package geecache

import (
	"fmt"
	"geecache/consistenthash"
	pb "geecache/geecachepb"
	"github.com/golang/protobuf/proto"
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
// 既具备了提供 HTTP 服务的能力，
// 也具备了根据具体的 key，创建 HTTP 客户端从远程节点获取缓存值的能力。
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
	//例如 http://example.com/_geecache/
	baseURL string
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
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
	// 规定访问路径格式：/<basepath>/<groupname>/<key>
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "had request ", http.StatusBadRequest)
		return
	}

	groupName := parts[0]
	key := parts[1]
	fmt.Println(groupName, key)
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
	//将值作为原型消息写入响应主体
	//编码Http响应
	body, err := proto.Marshal(&pb.Response{Value: view.ByteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	//将缓存值作为httpResponse 的 body 返回。
	//w.Write(view.ByteSlice())
	w.Write(body)
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

func (h *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	u := fmt.Sprintf(
		"%v%v/%v",
		h.baseURL,
		url.QueryEscape(in.GetGroup()),
		url.QueryEscape(in.GetKey()),
	)
	//通过http的通信方式访问远程节点的地址并且获取返回值
	res, err := http.Get(u)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}
	//读取Body部分的所有内容
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}
	//解码http响应
	if err = proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	return nil
}

var _ PeerGetter = (*httpGetter)(nil)
