package client

import (
	"gebrpc/registry"
	"gebrpc/server"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

type DService struct {
	count int
	mu    sync.Mutex
}

func (d *DService) Increase() {
	d.mu.Lock()
	d.count++
	d.mu.Unlock()
}

func startServer2() (*server.Server, string) {
	l, err := net.Listen("tcp4", "")
	var conn io.ReadWriteCloser
	if err != nil {
		log.Fatal("start server error ", err)
	}
	log.Println("start rpc server on ", l.Addr().String())
	serv := server.NewServer()
	go func() {
		for {
			conn, err = l.Accept()
			if err != nil {
				log.Fatal(err)
			}
			serv.ServeConn(conn)
		}
	}()
	return serv, l.Addr().String()
}

func TestBalancedClient_New(t *testing.T) {
	se, err := registry.StartServer()
	assert.Nil(t, err)

	c1, err := NewBalancedClient(se.Addr(), "", "", nil)
	assert.Nil(t, err)

	c11 := c1.(*BalancedClient)
	log.Printf(c11.registry.Name)

	ser1, addr1 := startServer2()
	ser2, addr2 := startServer2()
	ser3, addr3 := startServer2()
	d1, d2, d3 := &DService{}, &DService{}, &DService{}

	err = ser1.Register(d1)
	assert.Nil(t, err)
	_, err = NewBalancedClient(se.Addr(), "DService", addr1, nil)
	assert.Nil(t, err)
	err = ser2.Register(d2)
	assert.Nil(t, err)
	_, err = NewBalancedClient(se.Addr(), "DService", addr2, nil)
	assert.Nil(t, err)
	err = ser3.Register(d3)
	assert.Nil(t, err)
	_, err = NewBalancedClient(se.Addr(), "DService", addr3, nil)
	assert.Nil(t, err)
	se.PrintInfo()
	time.Sleep(100 * time.Millisecond)

	sers, err := c11.registry.GetServiceAdders("DService")
	assert.Nil(t, err)
	assert.Equal(t, 3, len(sers))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			err = c1.Call("DService:Increase", nil)
			assert.Nil(t, err)
			wg.Done()
		}()
	}
	wg.Wait()
	log.Printf("[d1.count,d2.count,d3.count]=[%d,%d,%d]", d1.count, d2.count, d3.count)
	assert.Equal(t, 10, d1.count+d2.count+d3.count)

}
