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
}
