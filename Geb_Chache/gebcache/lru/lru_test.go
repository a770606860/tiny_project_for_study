package lru

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type String string

func (d String) Len() int {
	return len(d)
}

func TestGet(t *testing.T) {
	lru := New(int64(1e6), nil)
	lru.Add("key1", String("1234"))
	v, ok := lru.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, "1234", string(v.(String)))

	_, ok = lru.Get("key2")
	assert.False(t, ok)
}

func TestCache_RemoveOldest(t *testing.T) {
	k1, k2, k3 := "key1", "key2", "k3"
	v1, v2, v3 := "value1", "value2", "v3"
	cap := len(k1 + k2 + v1 + v2)
	lru := New(int64(cap), nil)
	lru.Add(k1, String(v1))
	lru.Add(k2, String(v2))
	lru.Add(k3, String(v3))

	_, ok := lru.Get("key1")
	assert.False(t, ok)
	assert.Equal(t, 2, lru.TotalElem())

	lru.Add(k2, String("v2"))
	v, ok := lru.Get(k2)
	assert.Equal(t, "v2", string(v.(String)))
	assert.Equal(t, 2, lru.TotalElem())
}

func TestOnEvicted(t *testing.T) {
	i := 0
	f := func(key string, value Value) {
		i++
	}
	lru := New(int64(10), f)
	lru.Add("key1", String("123456"))
	lru.Add("k2", String("va"))
	lru.Add("k3", String("va"))
	lru.Add("k4", String("va"))
	lru.Add("k5", String("va"))

	assert.Equal(t, 3, i)
}
