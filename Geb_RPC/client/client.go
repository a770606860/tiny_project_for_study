package client

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
	"sync"
	"time"
)

// TODO 支持取消请求操作
type Call struct {
	Seq          uint64        // 序列号
	TargetMethod string        // <service>:<methodName>
	Args         []interface{} // 参数
	Reply        interface{}   // 返回数据的指针
	Error        error
	Done         chan *Call // 通知调用方调用完成，必须是缓存chan，以免阻塞client
}

var TimeOutError = errors.New("connect or negotiate server time out")

func (call *Call) done() {
	select {
	case call.Done <- call:
	default:
		log.Panic("rpc client: error must be buffed chan!!!")
	}
}

type Client struct {
	cc       codec.Codec
	sending  sync.Mutex
	mu       sync.Mutex
	pending  map[uint64]*Call // 待发送和待接收数据的call
	closing  bool
	shutdown bool
	seq      uint64
}

var _ io.Closer = (*Client)(nil)

var ErrShutdown = errors.New("connection is shut down")

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing {
		return ErrShutdown
	}
	c.closing = true
	return c.cc.Close()
}

func (c *Client) IsAvailable() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.shutdown && !c.closing
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closing || c.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = c.seq
	c.pending[call.Seq] = call
	c.seq++
	return call.Seq, nil
}

func (c *Client) removeCall(seq uint64) *Call {
	c.mu.Lock()
	defer c.mu.Unlock()
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

func (c *Client) terminateCalls(err error) {
	// 发送锁是为了保持语义的完整
	// TODO： defer的执行顺序未确认，理论上先解锁mu再sending
	c.sending.Lock()
	defer c.sending.Unlock()
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.shutdown == true {
		return
	}
	c.shutdown = true
	for _, call := range c.pending {
		call.Error = err
		call.done()
	}
}

func (c *Client) receive() {
	var err error
	for err == nil {
		var resp codec.Response
		// 有些编解码器编码后不带类型信息例如JSON，而有些则携带比如Gob。
		// 这些差异由具体的codec的实现负责处理，例如：
		// Gob编解器码需要注册类型；
		// Json解码器需要将Request拆开为head, body进行发送，从head获取足够的信息后再利用这些信息解码body；
		if err = c.cc.ReadResponse(&resp); err != nil {
			break
		}
		call := c.removeCall(resp.Seq)
		// 确保call未失效
		if call != nil {
			if resp.Err != "" {
				call.Error = errors.New(resp.Err)
			}
			call.Reply = resp.Replyv
			call.done()
		}
	}
	c.terminateCalls(err)
}

func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()

	seq, err := c.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	// 传递之前将参数解引用
	var argd []interface{}
	for _, arg := range call.Args {
		var argv interface{}
		if reflect.TypeOf(arg).Kind() == reflect.Ptr {
			argv = reflect.ValueOf(arg).Elem().Interface()
		} else {
			argv = arg
		}
		argd = append(argd, argv)
	}

	req := codec.Request{Seq: seq, Argv: argd, TargetMethod: call.TargetMethod}
	if err := c.cc.WriteRequest(&req); err != nil {
		// 发送失败，直接返回call
		call := c.removeCall(seq)
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

func (c *Client) Go(targetMethod string, replyMark interface{}, args ...interface{}) *Call {
	call := &Call{
		TargetMethod: targetMethod,
		Args:         args,
		Done:         make(chan *Call, 10),
		Reply:        replyMark,
	}
	c.send(call)
	return call
}

func (c *Client) Call(targetMethod string, replyMark interface{}, args ...interface{}) *Call {
	call := <-c.Go(targetMethod, replyMark, args...).Done
	return call
}

func (c *Client) dail(addr string, timeOut time.Duration, option *protocol.Option) error {
	finish := make(chan error, 1)
	connCh := make(chan net.Conn, 1)
	go func() {
		conn, err := net.DialTimeout("tcp", addr, timeOut)
		connCh <- conn
		if err == nil {
			log.Printf("rpc client: connection established, negociating")
		}
		defer func() {
			finish <- err
		}()
		if err != nil {
			return
		}
		err = json.NewEncoder(conn).Encode(option)
		log.Println("rpc client: option sent")
		if err != nil {
			log.Printf("rpc client: %v", err)
			return
		}
		resp := make([]byte, 2)
		_, err = conn.Read(resp)
		if err != nil {
			log.Printf("rpc client: %v", err)
			return
		}
		if string(resp) != "ok" {
			err = fmt.Errorf("server refused %s", resp)
			return
		}
		cc := codec.NewGobCodec(conn)
		c.cc = cc
		return
	}()
	select {
	case <-time.After(timeOut):
		conn := <-connCh
		if conn != nil {
			closeCloseable(conn)
		}
		return TimeOutError
	case err := <-finish:
		if c.cc != nil && err != nil {
			closeCloseable(c.cc)
		}
		return err
	}
}

func closeCloseable(closeable io.Closer) {
	err := closeable.Close()
	if err != nil {
		log.Printf("rpc client: close error %v", err)
	}
}

func NewClient(addr string, timeOut time.Duration, option ...*protocol.Option) (*Client, error) {
	c := &Client{pending: make(map[uint64]*Call)}
	op := parseOption(option...)
	if err := c.dail(addr, timeOut, op); err != nil {
		return nil, err
	}
	go c.receive()
	return c, nil
}

func parseOption(option ...*protocol.Option) *protocol.Option {
	if len(option) != 1 || option[0] == nil {
		return protocol.DefaultOption
	}
	return option[0]
}
