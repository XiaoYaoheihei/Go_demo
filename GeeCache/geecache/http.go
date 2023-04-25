package geecache

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const defaultBasePath = "/_geecache/"

type HTTPPool struct {
	//用来记录自己地址
	self string
	//作为节点间通讯地址的前缀
	basePath string
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: defaultBasePath,
	}
}

func (h *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", h.self, fmt.Sprintf(format, v...))
}

// 服务端的实现逻辑
func (h *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//首先检查访问的路由是否有规定前缀
	if !strings.HasPrefix(r.URL.Path, h.basePath) {
		panic("HTTPPool serving unexpected path: " + r.URL.Path)
	}
	//日志打印出相应的信息
	h.Log("%s %s", r.Method, r.URL.Path)
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
