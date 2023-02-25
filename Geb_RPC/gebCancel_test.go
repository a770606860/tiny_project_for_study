package gebrpc

import (
	"gebrpc/client"
	"gebrpc/server"
	"github.com/stretchr/testify/assert"
	"io"
	"log"
	"net"
	"testing"
	"time"
)

type ServiceSlow struct {
	name string
}

func (s *ServiceSlow) SetName(name string) {
	time.Sleep(time.Second)
	s.name = name
}
func (s *ServiceSlow) GetName() string {
	return s.name
}

func startServer2(addr chan string) {

	l, err := net.Listen("tcp", ":0")
	var conn io.ReadWriteCloser
	if err != nil {
		log.Fatal("start server error ", err)
	}
	log.Println("start rpc server on ", l.Addr().String())
	addr <- l.Addr().String()
	serv = server.NewServer()
	for {
		conn, err = l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		serv.ServeConn(conn)
	}
}

func Test_Cancel(t *testing.T) {
	// 启动客户端和服务器
	addr := make(chan string)
	go startServer(addr)
	c, err := client.NewClient(<-addr, time.Second)
	if err != nil {
		log.Fatal("New client error: ", err)
	}

	// 注册服务
	err = serv.Register(&ServiceSlow{})
	assert.Nil(t, err)
	err = c.Call("ServiceSlow:SetName", nil, "weiwei")
	assert.Nil(t, err)
	now := time.Now()
	err = c.CallUntil(500*time.Millisecond, "ServiceSlow:SetName", nil, "feifei")
	assert.Equal(t, client.ErrWaitingForReceiving, err)
	assert.True(t, time.Since(now) < 700*time.Millisecond)
	call := c.Go("ServiceSlow:SetName", nil, "shanshan")
	call.WaitFor(1600 * time.Millisecond)
	assert.Nil(t, call.Error)
	var str string
	call = c.Go("ServiceSlow:GetName", &str)
	<-call.Done
	assert.Nil(t, call.Error)
	assert.Equal(t, "shanshan", str)
	assert.Equal(t, "shanshan", *(call.Reply.(*string)))

	str = "x"
	err = c.Call("ServiceSlow:GetName", &str)
	assert.Equal(t, "shanshan", str)

}
