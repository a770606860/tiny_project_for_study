package gebcache

import (
	"fmt"
	"gebcache/singleflight"
	"log"
	"sync"
)

type Getter interface {
	Get(key string) ([]byte, error)
}

type GetterFunc func(key string) ([]byte, error)

func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

type Group struct {
	name      string
	getter    Getter
	mainCache cache
	Peers     PeerGetter
	sg        singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroupWithFunc(name string, cacheBytes int64, peerGetter PeerGetter, f GetterFunc) *Group {
	return NewGroup(name, cacheBytes, peerGetter, f)
}

func NewGroup(name string, cacheBytes int64, peerGetter PeerGetter, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache{cacheBytes: cacheBytes},
		Peers:     peerGetter,
	}
	mu.Lock()
	groups[name] = g
	mu.Unlock()
	return g
}

func GetGroup(name string) *Group {
	mu.RLock()
	defer mu.RUnlock()
	return groups[name]
}

func (g *Group) Get(key string) ([]byte, error) {
	if key == "" {
		return nil, fmt.Errorf("key is required")
	}
	if v, ok := g.mainCache.get(key); ok {
		log.Println("[GeeCache] hit")
		return v.Data(), nil
	}

	v, err := g.sg.Do(key, func() (interface{}, error) {
		return g.load(key)
	})

	return v.([]byte), err
}

func (g *Group) load(key string) ([]byte, error) {

	if g.Peers != nil {
		data, err := g.Peers.Get(g.name, key)
		if err != nil {
			// 如果远端出现错误，尝试从本地获取
			return g.getLocally(key)
		}
		if len(data) != 0 {
			return data, nil
		}
	}
	return g.getLocally(key)
}

// TODO:如果本地没有数据，应该防止再次从本地获取数据
func (g *Group) getLocally(key string) ([]byte, error) {
	data, err := g.getter.Get(key)
	if err != nil {
		return nil, err
	}
	value := ByteView{cloneBytes(data)}
	g.populateCache(key, value)
	return value.Data(), nil
}

func (g *Group) populateCache(key string, value ByteView) {
	g.mainCache.add(key, value)
}
