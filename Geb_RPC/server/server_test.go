package server

import (
	"gebrpc/client"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

type ServiceVerySlow struct {
	name string
}

func (s *ServiceVerySlow) SetName(name string) {
	time.Sleep(time.Second)
	s.name = name
}
func (s *ServiceVerySlow) GetName() string {
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
	c, err := client.NewClient(<-addr, time.Second)
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

// 测试心跳代码
// 需要在客户更改客户端代码，禁止其发送心跳或者，调高心跳间隔
//Client.dail函数修改如下，将心跳间隔增加到3倍：
//cc := codec.NewGobCodec(conn)
//c.cc = cc
//if resp.Tick != 0 {
//go c.ticker(resp.Tick*3)
//}
func Test_Tick(t *testing.T) {
	// 启动客户端和服务器
	addr := make(chan string)
	go startServer(addr)
	c, err := client.NewClient(<-addr, time.Second)
	if err != nil {
		log.Fatal("New client error: ", err)
	}

	// 注册服务
	err = serv.Register(&ServiceVerySlow{})
	assert.Nil(t, err)

	var str string
	//err = c.Call("ServiceVerySlow:GetName", &str)
	//assert.Nil(t, err)
	//assert.Equal(t, "", str)
	time.Sleep(9 * time.Second)
	err = c.Call("ServiceVerySlow:GetName", &str)
	assert.NotNil(t, err)
	log.Println(err)
}
