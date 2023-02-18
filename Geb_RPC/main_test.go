package main

import (
	"gebrpc/client"
	"gebrpc/server"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net"
	"testing"
)

var serv *server.Server

type StudentService struct {
	name string
	age  int
}

func (s *StudentService) SetName(name string) {
	s.name = name
}
func (s *StudentService) GetName() string {
	return s.name
}

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	var conn io.ReadWriteCloser
	if err != nil {
		log.Fatal("start server error ", err)
	}
	log.Println("start rpc server on ", l.Addr().String())
	addr <- l.Addr().String()
	conn, err = l.Accept()
	if err != nil {
		log.Fatal(err)
	}
	serv = server.NewServer()
	serv.ServeConn(conn)
}

func Test_Main(t *testing.T) {
	// 启动客户端和服务器
	addr := make(chan string)
	go startServer(addr)
	c, err := client.NewClient(<-addr)
	if err != nil {
		log.Fatal("New client error: ", err)
	}

	// test1，不注册任何服务，所以会返回错误
	call1 := c.Go("foo", "xiaobai")
	<-call1.Done
	assert.NotNil(t, call1.Error)

	// 注册服务
	err = serv.Register(&StudentService{})
	assert.Nil(t, err)
	call3 := c.Call("StudentService:SetName", "weiwei")
	assert.Nil(t, call3.Error)
	call3 = c.Call("StudentService:GetName")
	assert.Nil(t, call3.Error)
	assert.Equal(t, "weiwei", call3.Reply.(string))
}
