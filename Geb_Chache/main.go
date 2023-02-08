package main

import (
	"fmt"
	"gebcache"
	"log"
	"net/http"
	"os"
)

func main() {
	urls := os.Args[1:]
	g := newTestData(urls[0], urls[1], urls[2])
	path := urls[0][7:]

	log.Fatal(http.ListenAndServe(path, g.Peers.(*gebcache.HTTPPool)))
}

func newTestData(self, url2, url3 string) *gebcache.Group {
	var db = map[string]string{
		"tom":  "630",
		"jack": "589",
		"sam":  "567",
	}
	g := gebcache.NewGroupWithFunc("scores", 2<<10, nil, func(key string) ([]byte, error) {
		log.Println("[db] search key", key)
		if v, ok := db[key]; ok {
			return []byte(v), nil
		}
		return nil, fmt.Errorf("%s not exist", key)
	})
	peer := gebcache.NewHTTPPool(self, "", nil)
	peer.AddPeers(url3, url2, self)
	g.Peers = peer
	return g
}
