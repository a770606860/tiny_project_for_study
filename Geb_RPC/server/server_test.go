package server

import (
	"gebrpc/client"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

type ServiceVerySlow struct {
	name string
	mu   sync.Mutex
}

func (s *ServiceVerySlow) SetName(name string) {
	time.Sleep(time.Second)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
}
func (s *ServiceVerySlow) GetName() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

var serv *Server

func startServer(addr chan string) {

	l, err := net.Listen("tcp", ":0")
	var conn io.ReadWriteCloser
	if err != nil {
		log.Fatal("start server error ", err)
	}
	log.Println("start rpc server on ", l.Addr().String())
	addr <- l.Addr().String()
	serv = NewServer()
	for {
		conn, err = l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		serv.ServeConn(conn)
	}
}

func Test_TimeOut(t *testing.T) {
	// 启动客户端和服务器
	addr := make(chan string)
	go startServer(addr)
	c, err := client.NewClientTimeOut(<-addr, time.Second, nil)
	if err != nil {
		log.Fatal("New client error: ", err)
	}

	// 注册服务
	err = serv.Register(&ServiceVerySlow{})
	assert.Nil(t, err)
	err = c.Call("ServiceVerySlow:SetName", nil, "weiwei")
	assert.Equal(t, ErrServiceTimeOut, err)
	var str string
	err = c.Call("ServiceVerySlow:GetName", &str)
	assert.Nil(t, err)
	assert.Equal(t, "", str)
	time.Sleep(2 * time.Second)
	err = c.Call("ServiceVerySlow:GetName", &str)
	assert.Nil(t, err)
	assert.Equal(t, "weiwei", str)
	time.Sleep(5 * time.Second)
	err = c.Call("ServiceVerySlow:GetName", &str)
	assert.Nil(t, err)
	assert.Equal(t, "weiwei", str)
}

type DService struct {
	count int
	mu    sync.Mutex
}

func (d *DService) Increase() {
	d.mu.Lock()
	d.count++
	d.mu.Unlock()
}

func (d *DService) GetCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.count
}

func startServer2() (*Server, string) {
	l, err := net.Listen("tcp4", "")
	var conn io.ReadWriteCloser
	if err != nil {
		log.Fatal("start server error ", err)
	}
	log.Println("start rpc server on ", l.Addr().String())
	serv := NewServer()
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

func TestDService(t *testing.T) {
	ser1, addr1 := startServer2()
	ser2, addr2 := startServer2()
	ser3, addr3 := startServer2()
	d1 := &DService{}
	d2 := &DService{}
	d3 := &DService{}

	err := ser1.Register(d1)
	assert.Nil(t, err)
	err = ser2.Register(d2)
	assert.Nil(t, err)
	err = ser3.Register(d3)
	assert.Nil(t, err)
	se1, _ := ser1.services.Load("DService")
	se2, _ := ser2.services.Load("DService")

	se3, _ := ser3.services.Load("DService")

	log.Printf("__________%x,%x,%x", se1.(*service).rcvr.Pointer(), se2.(*service).rcvr.Pointer(), se3.(*service).rcvr.Pointer())
	////log.Printf("++++++++++%x,%x,%x", se11.methods["Increase"].)
	//se11 := se1.(*service)
	//se22 := se2.(*service)
	//se33 := se3.(*service)
	//se11.call(se11.methods["Increase"])
	//se22.call(se22.methods["Increase"])
	//se33.call(se33.methods["Increase"])
	//assert.Equal(t, 1, d1.GetCount())
	//assert.Equal(t, 1, d2.GetCount())
	//assert.Equal(t, 1, d3.GetCount())

	c1, err := client.NewClient(addr1, nil)
	assert.Nil(t, err)
	c2, err := client.NewClient(addr2, nil)
	assert.Nil(t, err)
	c3, err := client.NewClient(addr3, nil)
	assert.Nil(t, err)

	err = c1.Call("DService:Increase", nil)
	assert.Nil(t, err)
	err = c2.Call("DService:Increase", nil)
	assert.Nil(t, err)
	err = c3.Call("DService:Increase", nil)
	se1, _ = ser1.services.Load("DService")
	se2, _ = ser2.services.Load("DService")

	se3, _ = ser3.services.Load("DService")

	log.Printf("+++++++++%x,%x,%x", se1.(*service).rcvr.Pointer(), se2.(*service).rcvr.Pointer(), se3.(*service).rcvr.Pointer())
	assert.Nil(t, err)
	assert.Equal(t, 1, d1.GetCount())
	assert.Equal(t, 1, d2.GetCount())
	assert.Equal(t, 1, d3.GetCount())

}

//// 测试心跳代码
//// 需要在客户更改客户端代码，禁止其发送心跳或者，调高心跳间隔
////DefaultClient.dail函数修改如下，将心跳间隔增加到3倍：
////cc := codec.NewGobCodec(conn)
////c.cc = cc
////if resp.Tick != 0 {
////go c.ticker(resp.Tick*3)
////}
//func Test_Tick(t *testing.T) {
//	// 启动客户端和服务器
//	addr := make(chan string)
//	go startServer(addr)
//	c, err := client.NewClientTimeOut(<-addr, time.Second)
//	if err != nil {
//		log.Fatal("New client error: ", err)
//	}
//
//	// 注册服务
//	err = serv.register(&ServiceVerySlow{})
//	assert.Nil(t, err)
//
//	var str string
//	err = c.Call("ServiceVerySlow:GetName", &str)
//	assert.Nil(t, err)
//	assert.Equal(t, "", str)
//	time.Sleep(9 * time.Second)
//	err = c.Call("ServiceVerySlow:GetName", &str)
//	assert.NotNil(t, err)
//	log.Println(err)
//}
