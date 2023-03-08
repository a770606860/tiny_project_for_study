package registry

import (
	"github.com/stretchr/testify/assert"
	"log"
	"net"
	"testing"
	"time"
)

func newClient(name, serverAddr string) (*RegisterClient, error) {
	l, err := net.Listen("tcp4", "")
	if err != nil {
		log.Printf("testing new client failed: 1")
		return nil, err
	}
	c, err := NewClient(name, l.Addr().String(), serverAddr, time.Second*1)
	if err != nil {
		log.Printf("testing new client failed: 2")
		return nil, err
	}
	return c, nil
}

// 单独测试该方法
func TestRegisterAndResign(t *testing.T) {
	se, err := StartServer()
	assert.Nil(t, err)
	c1, err := newClient("serv1", se.addr)
	assert.Nil(t, err)
	se.seMu.Lock()
	assert.Equal(t, 1, len(se.idToService))
	assert.Equal(t, 1, len(se.services))
	se.seMu.Unlock()
	se.clMu.Lock()
	assert.Equal(t, 1, len(se.clients))
	se.clMu.Unlock()

	c2, err := newClient("serv2", se.addr)
	assert.Nil(t, err)

	time.Sleep(5 * time.Second)
	se.seMu.Lock()
	assert.Equal(t, 2, len(se.idToService))
	assert.Equal(t, 2, len(se.services))
	se.seMu.Unlock()
	se.clMu.Lock()
	assert.Equal(t, 2, len(se.clients))
	se.clMu.Unlock()

	err = c2.Close()
	assert.Nil(t, err)
	se.seMu.Lock()
	assert.Equal(t, 1, len(se.idToService))
	assert.Equal(t, 1, len(se.services))
	se.seMu.Unlock()
	se.clMu.Lock()
	assert.Equal(t, 1, len(se.clients))
	se.clMu.Unlock()

	err = c1.resign()
	se.seMu.Lock()
	assert.Equal(t, 0, len(se.idToService))
	assert.Equal(t, 0, len(se.services))
	se.seMu.Unlock()
	se.clMu.Lock()
	assert.Equal(t, 0, len(se.clients))
	se.clMu.Unlock()
}

// 单独测试该方法
func TestGetServices(t *testing.T) {
	se, err := StartServer()
	assert.Nil(t, err)
	c1, err := newClient("serv1", se.addr)
	assert.Nil(t, err)

	ser, _ := c1.GetServiceAdders("serv2")
	assert.Equal(t, 0, len(ser))

	//添加服务
	c2, err := newClient("serv2", se.addr)
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 100)
	ser, _ = c1.GetServiceAdders("serv2")
	assert.Equal(t, 1, len(ser))

	//再添加一个服务
	c3, err := newClient("serv2", se.addr)
	assert.Nil(t, err)
	c4, err := newClient("serv2", se.addr)
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 100)
	ser, _ = c1.GetServiceAdders("serv2")
	assert.Equal(t, 3, len(ser))

	// 删除服务
	err = c2.Close()
	assert.Nil(t, err)
	time.Sleep(time.Millisecond * 100)
	ser, _ = c1.GetServiceAdders("serv2")
	time.Sleep(time.Millisecond * 100)
	assert.Equal(t, 2, len(ser))

	ser, _ = c3.GetServiceAdders("serv1")
	assert.Equal(t, 1, len(ser))

	// 更改c4的id为不存在的id，模拟心跳失败
	c4.mu.Lock()
	c4.id = 1001
	c4.mu.Unlock()
	time.Sleep(5 * time.Second)
	ser, _ = c1.GetServiceAdders("serv2")
	assert.Equal(t, 1, len(ser))

	ser, _ = c3.GetServiceAdders("serv1")
	assert.Equal(t, 1, len(ser))
	se.printInfo()
}
