package gebcache

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
)

var db = map[string]string{
	"tom":  "630",
	"jack": "589",
	"sam":  "567",
}

func TestGet(t *testing.T) {
	loadCounts := map[string]int{}
	g := NewGroupWithFunc("scores", 2<<10, nil, func(key string) ([]byte, error) {
		log.Println("[DB] search key", key)
		if v, ok := db[key]; ok {
			loadCounts[key]++
			return []byte(v), nil
		}
		return nil, fmt.Errorf("%s not exist", key)
	})
	for k, _ := range db {
		_, err := g.Get(k)
		assert.Nil(t, err)
	}
	for k, v := range db {
		d, err := g.Get(k)
		assert.Nil(t, err)
		assert.Equal(t, v, string(d))
	}
	d, err := g.Get("weiwei")
	assert.Error(t, err)
	assert.Empty(t, d)

	for _, v := range loadCounts {
		assert.Equal(t, 1, v)
	}
}
