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
	"time"
)

var ErrServiceTimeOut = errors.New("service time out")

type Server struct {
	services sync.Map
}

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Register(rcvr interface{}) error {
	se := newService(rcvr)
	if _, dup := s.services.LoadOrStore(se.name, se); dup {
		return errors.New("service already defined: " + se.name)
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
		err = fmt.Errorf("method format ill-formed: %s, except:<service>:<name>", req.TargetMethod)
		return err
	}
	svci, ok := s.services.Load(name[0])
	if !ok {
		err = fmt.Errorf("service %s not avialible", name[0])
		return err
	}
	svc := svci.(*service)
	mtype := svc.methods[name[1]]
	if mtype == nil {
		err = fmt.Errorf("service %s has no method %s", name[0], name[1])
		return err
	}

	var argvs []reflect.Value

	for i, arg := range req.Argv {
		if t := mtype.argType[i+1]; t.Kind() == reflect.Ptr &&
			reflect.TypeOf(arg).Kind() != reflect.Ptr {
			p := reflect.New(t.Elem())
			p.Elem().Set(reflect.ValueOf(arg))
			argvs = append(argvs, p)
		} else {
			argvs = append(argvs, reflect.ValueOf(arg))
		}
	}
	if re := svc.call(mtype, argvs...); re != nil {
		replyv := reflect.ValueOf(re)
		if replyv.Kind() == reflect.Ptr {
			resp.Replyv = replyv.Elem().Interface()
		} else {
			resp.Replyv = replyv.Interface()
		}
	}

	return nil
}

func (s *Server) ServeConn(conn io.ReadWriteCloser) {
	var opt protocol.Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Printf("rpc server: options error %v", err)
		closeCloseable(conn)
		return
	}
	if opt.MagicNumber != protocol.MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		closeCloseable(conn)
		return
	}
	log.Printf("rpc server: client approved")
	var tick int64

	var c codec.Codec
	switch opt.CodecType {
	case codec.GobType:
		c = codec.NewGobCodec(conn)
		tick = 2
	default:
		log.Println("rpc server: unsupported codec type")
		closeCloseable(conn)
		return
	}
	if err := json.NewEncoder(conn).Encode(protocol.ServerReply{Code: "ok", Tick: tick}); err != nil { // 发送确认信息到客户端失败
		log.Printf("rpc server: connection err %s", err)
		closeCloseable(conn)
		return
	}
	// 心跳检测
	var ch chan struct{}
	if tick != 0 {
		ch = make(chan struct{}, 10)
		defer close(ch)
		go heartListen(tick, ch, c)
	}

	s.serveCodec(c, ch) // 死循环
}

func heartListen(tick int64, ch chan struct{}, c codec.Codec) {
	tick = tick * 2
	for {
		select {
		case <-time.After(time.Duration(tick) * time.Second):
			closeCloseable(c)
			if conn, ok := c.GetConn().(net.Conn); ok {
				log.Printf("rpc server: failed receive client tick  %s", conn.RemoteAddr())
			}
			return
		case _, ok := <-ch:
			if !ok {
				return
			}
		}
	}
}

func (s *Server) serveCodec(c codec.Codec, tick chan struct{}) {
	var wg sync.WaitGroup
	resultCh := make(chan *codec.Response, 10)
	go s.sendResponse(c, resultCh)
loop:
	for {
		// 写损坏，停止处理
		select {
		case <-c.Failed():
			break loop
		default:
		}
		// 读损坏，停止处理
		req, err := s.readRequest(c)
		if err != nil {
			break loop
		}
		// 当targetMethod为nil时，为心跳包
		if req.TargetMethod == "" && tick != nil {
			tick <- struct{}{}
			continue
		}

		wg.Add(1)
		go func() {
			s.handleRequest(req, resultCh, err)
			wg.Done()
		}()
	}
	wg.Wait()
	close(resultCh)
	if cc, ok := c.GetConn().(net.Conn); ok {
		log.Printf("rpc server: close connection [%s:%s]", cc.RemoteAddr(), cc.LocalAddr())
	}
	closeCloseable(c)
}

func (s *Server) readRequest(c codec.Codec) (*codec.Request, error) {
	var r codec.Request
	// 有些编解码器编码后不带类型信息例如JSON，而有些则携带比如Gob。
	// 这些差异由具体的codec的实现负责处理，例如：
	// Gob编解器码需要注册类型；
	// Json解码器需要将Request拆开为head, body进行发送，从head获取类型信息后再利用这些信息解码body；
	err := c.ReadRequest(&r)
	if err == nil {
		log.Printf("rpc server: data recived Seq=%d", r.Seq)
	} else if err != io.EOF {
		log.Printf("rpc server: recive data Seq=%d error %s", r.Seq, err)
	}
	return &r, err
}

// 超时处理，每个方法执行时间不得超过700ms，如果超过，则返回处理超时错误
func (s *Server) handleRequest(req *codec.Request, ch chan *codec.Response, err error) {
	var resp codec.Response
	resp.Seq = req.Seq
	if err != nil {
		resp.Err = err.Error()
		return
	}

	finished := make(chan struct{})
	go func() {
		err = s.service(req, &resp)
		close(finished)
	}()

	select {
	case <-time.After(700 * time.Millisecond):
		err = ErrServiceTimeOut
		log.Printf("rpc server: %s timeout, please check", req.TargetMethod)
	case <-finished:
		if err != nil {
			log.Printf("rpc server: error %v", err)
		}
	}
	if err != nil {
		resp.Err = err.Error()
	}
	ch <- &resp
}

func (s *Server) sendResponse(c codec.Codec, ch chan *codec.Response) {
	for r := range ch {
		err := c.WriteResponse(r)
		if err != nil {
			log.Printf("rpc server: send error Seq=%d %s", r.Seq, err)
			closeCloseable(c)
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

func closeCloseable(closeable io.Closer) {
	if err := closeable.Close(); err != nil {
		log.Printf("rpc server: close error %v", err)
	}
}
