package client

import (
	"github.com/stretchr/testify/assert"
	"log"
	"testing"
	"time"
)

type ServiceSlow struct {
	name string
}

func (s *ServiceSlow) SetName(name string) {
	time.Sleep(400 * time.Millisecond)
	s.name = name
}
func (s *ServiceSlow) GetName() string {
	return s.name
}

func Test_Cancel(t *testing.T) {
	// 启动客户端和服务器
	addr := make(chan string)
	go startServer(addr)
	c, err := NewClientTimeOut(<-addr, time.Second)
	if err != nil {
		log.Fatal("New client error: ", err)
	}

	// 注册服务
	err = serv.Register(&ServiceSlow{})
	assert.Nil(t, err)
	err = c.Call("ServiceSlow:SetName", nil, "weiwei")
	assert.Nil(t, err)
	now := time.Now()
	err = c.CallUntil(200*time.Millisecond, "ServiceSlow:SetName", nil, "feifei")
	assert.Equal(t, ErrWaitingForReceiving, err)
	assert.True(t, time.Since(now) < 300*time.Millisecond)
	call := c.Go("ServiceSlow:SetName", nil, "shanshan")
	call.WaitFor(800 * time.Millisecond)
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
