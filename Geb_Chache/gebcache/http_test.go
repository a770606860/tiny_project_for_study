package gebcache

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPPool_ServerHTTP(t *testing.T) {
	var db = map[string]string{
		"tom":  "630",
		"jack": "589",
		"sam":  "567",
	}
	NewGroupWithFunc("scores", 2<<10, nil, func(key string) ([]byte, error) {
		log.Println("[db] search key", key)
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("%s not exist", key)
	})
	pool := NewHTTPPool("localhost:9999", "/geb_cache/", nil)
	r := httptest.NewRequest("GET", "/none", nil)
	w := httptest.NewRecorder()
	assert.Panics(t, func() { pool.ServeHTTP(w, r) })
	r = httptest.NewRequest("GET", "/geb_cache/scores/tom", nil)
	w = httptest.NewRecorder()
	pool.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "630", w.Body.String())
}
