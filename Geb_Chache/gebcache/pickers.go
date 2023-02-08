// 一些picker算法的实现
package gebcache

import (
	"hash/crc32"
	"sort"
	"strconv"
)

const (
	defaultReplicas = 10
)

type Picker interface {
	Add(key ...string)
	Pick(key string) string
}

type ConsistentHashPicker struct {
	hash     Hash
	replicas int
	keys     []int
	hashMap  map[int]string
}

type Hash func(data []byte) uint32

func newCSHPicker(replicas int, fn Hash) *ConsistentHashPicker {
	if replicas == 0 {
		replicas = defaultReplicas
	}
	m := &ConsistentHashPicker{
		replicas: replicas,
		hash:     fn,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// 添加分布式节点
func (m *ConsistentHashPicker) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i <= m.replicas; i++ {
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			m.keys = append(m.keys, hash)
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

func (m *ConsistentHashPicker) Pick(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	return m.hashMap[m.keys[idx%len(m.keys)]]
}
