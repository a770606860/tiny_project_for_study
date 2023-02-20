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

var serv *server.Server

type StudentService struct {
	name     string
	age      int
	p        Parents
	siblings []string
}
type Parents struct {
	Mother, Father string
}

func (s *StudentService) SetName(name string) {
	s.name = name
}
func (s *StudentService) GetName() string {
	return s.name
}
func (s *StudentService) SetParents(p *Parents) {
	s.p = *p
}
func (s *StudentService) SetParents2(p Parents) {
	s.p = p
}
func (s *StudentService) GetParents() *Parents {
	return &s.p
}

func startServer(addr chan string) {

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

func Test_Main(t *testing.T) {
	// 启动客户端和服务器
	addr := make(chan string)
	go startServer(addr)
	c, err := client.NewClient(<-addr, time.Second)
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
	call3 := c.Call("StudentService:SetName", nil, "weiwei")
	assert.Nil(t, call3.Error)
	call3 = c.Call("StudentService:GetName", nil)
	assert.Nil(t, call3.Error)
	assert.Equal(t, "weiwei", call3.Reply.(string))
	call3 = c.Call("StudentService:SetParents", nil, &Parents{"a", "b"})
	assert.Nil(t, call3.Error)
	call3 = c.Call("StudentService:GetParents", Parents{})
	assert.Nil(t, call3.Error)
	//assert.Equal(t, Parents{"a", "b"}, call3.Reply.(Parents))
}

func Test_Timeout(t *testing.T) {
	addr := make(chan string)
	go startServer(addr)
	ad := <-addr
	c, err := client.NewClient(ad, 400*time.Millisecond)
	assert.Nil(t, err)
	if err != nil {
		log.Println(err)
	} else {
		c.Close()
	}
	c, err = client.NewClient(ad, time.Microsecond)
	assert.Equal(t, client.TimeOutError, err)

	c, err = client.NewClient(ad, 1*time.Second)
	if err != nil {
		log.Println(err)
	}
	assert.Nil(t, err)
	if c != nil {
		err = c.Close()
		assert.Nil(t, err)
	}

}

//// 修改代码后单独测试该函数
//func Test_timeout2(t *testing.T) {
//	// 启动客户端和服务器
//	addr := make(chan string)
//	go startServer(addr)
//	ad := <-addr
//	// 下面的测试需要在client.dail函数中添加sleep代码以测试协商过程过长是否能够即使返回
//	// // 		err = json.NewEncoder(conn).Encode(option)
//	//		if err != nil {
//	//			return
//	//		}
//	//		time.Sleep(500 * time.Millisecond)
//	//		resp := make([]byte, 2)
//	// //
//	_, err := client.NewClient(ad, 400*time.Millisecond)
//	if err != nil {
//		log.Println(err.Error())
//	}
//	assert.Equal(t, client.TimeOutError, err)
//
//	_, err = client.NewClient(ad, 800*time.Millisecond)
//	assert.Nil(t, err)
//}
