package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"gebrpc/codec"
	"gebrpc/protocol"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
)

type Server struct {
	services sync.Map
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Register(rcvr interface{}) error {
	se := newService(rcvr)
	if _, dup := s.services.LoadOrStore(se.name, se); dup {
		return errors.New("rpc server: service already defined: " + se.name)
	}
	return nil
}

func (s *Server) service(req *codec.Request, resp *codec.Response) (err error) {
	name := strings.Split(req.TargetMethod, ":")
	defer func() {
		if err != nil {
			resp.Err = fmt.Errorf("service/method not found").Error()
		}
	}()
	if len(name) != 2 || len(name[0]) == 0 || len(name[1]) == 0 {
		err = fmt.Errorf("rpc server: method format ill-formed: %s, except:<service>:<name>", req.TargetMethod)
		return err
	}
	svci, ok := s.services.Load(name[0])
	if !ok {
		err = fmt.Errorf("rpc server: service %s not avialible", name[0])
		return err
	}
	svc := svci.(*service)
	mtype := svc.methods[name[1]]
	if mtype == nil {
		err = fmt.Errorf("rpc server: service %s has no method %s", name[0], name[1])
		return err
	}

	var argvs []reflect.Value
	for _, arg := range req.Argv {
		argvs = append(argvs, reflect.ValueOf(arg))
	}
	resp.Replyv = svc.call(mtype, argvs...)
	return nil
}

func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	var opt protocol.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Println("rpc server: options error", err)
		_ = conn.Close()
		return
	}
	if opt.MagicNumber != protocol.MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		_ = conn.Close()
		return
	}
	log.Printf("rpc server: client approved")
	if _, err := conn.Write([]byte("ok")); err != nil { // 发送确认信息到客户端失败
		log.Printf("rpc server: connection err %s", err)
		_ = conn.Close()
	}

	var c codec.Codec
	switch opt.CodecType {
	case codec.GobType:
		c = codec.NewGobCodec(conn)
	default:
		log.Println("rpc server: unsupported codec type")
		_ = conn.Close()
		return
	}
	s.serveCodec(c)
}

func (s *Server) serveCodec(c codec.Codec) {
	var wg sync.WaitGroup
	resultCh := make(chan *codec.Response, 10)
	go s.sendResponse(c, resultCh)
loop:
	for {
		// 连接损坏则停止处理
		select {
		case <-c.Failed():
			break loop
		default:
		}
		req, err := s.readRequest(c)
		if err != nil && req == nil {
			break
		}
		wg.Add(1)
		go func() {
			s.handleRequest(req, resultCh, err)
			wg.Done()
		}()
	}
	wg.Wait()
	close(resultCh)
	_ = c.Close()
}

func (s *Server) readRequest(c codec.Codec) (*codec.Request, error) {
	var r codec.Request
	err := c.Read(&r)

	if err == nil {
		log.Printf("rpc server: data recived Seq=%d", r.Seq)
	} else if err != io.EOF {
		log.Printf("rpc server: recive data Seq=%d error %s", r.Seq, err)
	}
	return &r, err
}

func (s *Server) handleRequest(req *codec.Request, ch chan *codec.Response, err error) {
	var resp codec.Response
	resp.Seq = req.Seq
	if err != nil {
		resp.Err = err.Error()
		return
	}
	err = s.service(req, &resp)
	if err != nil {
		log.Println(err)
	}
	ch <- &resp

}

func readAsString(argv interface{}) string {
	return reflect.ValueOf(argv).String()
}

func (s *Server) sendResponse(c codec.Codec, ch chan *codec.Response) {
	for r := range ch {
		err := c.Write(r)
		if err != nil {
			log.Printf("rpc server: send error Seq=%d %s", r.Seq, err)
			close(c.Failed()) // 发送数据发生错误，表明conn损坏
			break
		} else {
			log.Printf("rpc server: data sended Seq=%d", r.Seq)
		}
	}
	drain(ch) // 清空通道，防止死锁
}

func drain(ch chan *codec.Response) {
	for range ch {
	}
}

func (s *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Println("rpc server: accept error:", err)
			return // 直接关闭服务器
		}
		go s.ServeConn(conn)
	}
}
