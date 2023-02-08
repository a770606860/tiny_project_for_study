package gebcache

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const defaultBasePath = "/gebcache/"

type HTTPPool struct {
	self     string
	basePath string
	mu       sync.Mutex
	picker   Picker
	getters  map[string]PeerGetter
}

type httpGetter struct {
	addr string
}

func (getter *httpGetter) Get(group, key string) ([]byte, error) {
	u := fmt.Sprintf("%v%v/%v", getter.addr, url.QueryEscape(group), url.QueryEscape(key))
	res, err := http.Get(u)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %v", res.Status)
	}
	bytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %v", err)
	}
	return bytes, nil
}

func (p *HTTPPool) AddPeers(key ...string) {
	for _, k := range key {
		p.getters[k] = &httpGetter{
			addr: fmt.Sprintf("%v%v", k, p.basePath),
		}
	}
	p.picker.Add(key...)
}

func (p *HTTPPool) Get(group, key string) ([]byte, error) {
	// TODO：如果各个节点保存的对端不一致那么导致请求在对端之间循环传递
	// TODO：可以在请求上添加一个计数，确保请求传递次数不超过1次
	ps := p.picker.Pick(key)
	if ps == p.self {
		return nil, nil
	}
	p.Log("Getting key [%s:%s] from server [%s]", group, key, ps)
	return p.getters[ps].Get(group, key)
}

func NewHTTPPool(self, basePath string, picker Picker) *HTTPPool {
	if len(basePath) == 0 {
		basePath = defaultBasePath
	}
	if basePath[len(basePath)-1] != '/' {
		basePath += "/"
	}
	if picker == nil {
		picker = newCSHPicker(0, nil)
	}
	return &HTTPPool{
		self:     self,
		basePath: basePath,
		picker:   picker,
		getters:  make(map[string]PeerGetter),
	}
}

func (p *HTTPPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

func (p *HTTPPool) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, p.basePath) {
		panic("HTTPPool serving unexpected path" + r.URL.Path)
	}
	p.Log("%s %s", r.Method, r.URL.Path)
	parts := strings.SplitN(r.URL.Path[len(p.basePath):], "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	groupName, key := parts[0], parts[1]
	group := GetGroup(groupName)
	// TODO 多节点下应该要有办法配置或者自动配置多个Group
	if group == nil {
		http.Error(w, "no such group: "+groupName, http.StatusNotFound)
		return
	}

	data, err := group.Get(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	if _, err = w.Write(data); err != nil {
		p.Log("%s", err.Error())
	}
}
