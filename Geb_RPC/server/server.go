package server

import (
	"encoding/json"
	"gebrpc/codec"
	"gebrpc/protocol"
	"io"
	"log"
	"net"
	"reflect"
	"sync"
)

type Server struct{}

func NewServer() *Server {
	return &Server{}
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
		req.Err = err
		wg.Add(1)
		go func() {
			s.handleRequest(req, resultCh)
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

func (s *Server) handleRequest(req *codec.Request, ch chan *codec.Response) {
	var resp codec.Response
	resp.Seq = req.Seq
	if req.Err != nil {
		resp.Err = req.Err
	}
	// 调用对应的处理函数进行处理，这里直接返回请求编号
	{
		// 暂时直接将参数和函数名返回
		fname := req.TargetMethod
		arg := readAsString(req.Argv)
		resp.Replyv = fname + ":" + arg
		ch <- &resp
	}
}

func readAsString(argv interface{}) string {
	return reflect.ValueOf(argv).String()
}

func (s *Server) sendResponse(c codec.Codec, ch chan *codec.Response) {
	for r := range ch {
		err := c.Write(r)
		if err != nil {
			log.Printf("rpc server: send error Seq=%d %s", r.Seq, err)
			close(c.Failed()) // 发送数据发生错误，表面通道损坏
		} else {
			log.Printf("rpc server: data sended Seq=%d", r.Seq)
		}
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
