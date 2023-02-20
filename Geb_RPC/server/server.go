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
	if _, err := conn.Write([]byte("ok")); err != nil { // 发送确认信息到客户端失败
		log.Printf("rpc server: connection err %s", err)
		closeCloseable(conn)
		return
	}

	var c codec.Codec
	switch opt.CodecType {
	case codec.GobType:
		c = codec.NewGobCodec(conn)
	default:
		log.Println("rpc server: unsupported codec type")
		closeCloseable(conn)
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
		if err != nil {
			break loop
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
	// Json解码器需要将Request拆开为head, body进行发送，从head获取足够的信息后再利用这些信息解码body；
	err := c.ReadRequest(&r)
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
		log.Printf("rpc server: error %v", err)
	}
	ch <- &resp

}

func (s *Server) sendResponse(c codec.Codec, ch chan *codec.Response) {
	for r := range ch {
		err := c.WriteResponse(r)
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

func closeCloseable(closeable io.Closer) {
	if err := closeable.Close(); err != nil {
		log.Printf("rpc server: close error %v", err)
	}
}
