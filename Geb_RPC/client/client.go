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

// 内存可见性由Done通道维护
type Call struct {
	Seq          uint64        // 序列号
	TargetMethod string        // <service>:<methodName>
	Args         []interface{} // 参数
	Reply        interface{}   // 返回数据的指针
	Error        error         // 错误，例如方法不存在，连接错误等
	Done         chan struct{} // 通知调用方调用完成，必须是缓存chan，以免阻塞client
	c            *Client
	mu           sync.Mutex
	status       int
}

const (
	NEW = iota
	RECEIVING
	FINISHED
)

var ErrTimeOut = errors.New("connect or negotiate server time out")
var ErrWaitingForReceiving = errors.New("call canceled while sending or waiting-for-receiving")
var ErrShutdown = errors.New("connection is shut down")

func (call *Call) done() {
	close(call.Done)
}

// TODO：后续可以尝试添加单向关闭功能
type Client struct {
	cc      codec.Codec
	sending sync.Mutex
	mu      sync.Mutex       // mu保护下面所有的字段
	pending map[uint64]*Call //
	seq     uint64
}

var _ io.Closer = (*Client)(nil)

// 等待指定时间后返回，如果call未完成则关闭call
func (call *Call) WaitFor(duration time.Duration) {
	select {
	case <-time.After(duration):
		call := call.c.removeCall(call.Seq)
		if call == nil {
			return
		}
		call.mu.Lock()
		if call.status == FINISHED {
			call.mu.Unlock()
			return
		} else if call.status == RECEIVING {
			call.Error = ErrWaitingForReceiving
		} else {
			call.Error = ErrShutdown
		}
		call.status = FINISHED
		call.mu.Unlock()
		call.done()
	case <-call.Done:
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.IsClosed() {
		return ErrShutdown
	}
	c.terminateCalls()
	return c.doClose()
}

func (c *Client) doClose() error {
	c.pending = nil
	return c.cc.Close()
}

// must hold mu
func (c *Client) IsClosed() bool {
	return c.pending == nil
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.IsClosed() {
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
	if c.IsClosed() {
		return nil
	}
	call := c.pending[seq]
	delete(c.pending, seq)
	return call
}

// must hold mu before call this func
// assert c.isClosed() false
func (c *Client) terminateCalls() {
	for _, call := range c.pending {
		call.mu.Lock()
		if call.status == FINISHED {
			call.mu.Unlock()
			continue
		} else if call.status == RECEIVING {
			call.Error = ErrWaitingForReceiving
		} else {
			call.Error = ErrShutdown
		}
		call.status = FINISHED
		call.mu.Unlock()
		call.done()

	}
	// 清空pending
	c.pending = make(map[uint64]*Call)
}

func (c *Client) receive() {
	var err error
	for err == nil {
		var resp codec.Response
		// 有些编解码器编码后不带类型信息例如JSON，而有些则携带比如Gob。
		// 这些差异由具体的codec的实现负责处理，例如：
		// Gob编解器码需要注册类型；
		// Json解码器需要将Request拆开为head, body进行发送，从head获取类型信息后再利用这些信息解码body；
		if err = c.cc.ReadResponse(&resp); err != nil {
			log.Printf("rpc client: read error [%s]", err)
			break
		}
		call := c.removeCall(resp.Seq)
		// 确保call未失效
		if call != nil {
			call.mu.Lock()
			if call.status == FINISHED {
				call.mu.Unlock()
				return
			} else if call.status != RECEIVING {
				call.mu.Unlock()
				// call.status must be receiving or finished
				panic("rpc client: internal logical error")
			}
			if resp.Err != "" {
				call.Error = errors.New(resp.Err)
			}
			if call.Reply != nil {
				re := reflect.ValueOf(call.Reply)
				re.Elem().Set(reflect.ValueOf(resp.Replyv))
			}
			call.status = FINISHED
			call.mu.Unlock()
			call.done()
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.terminateCalls()
}

func (c *Client) send(call *Call) {
	c.sending.Lock()
	defer c.sending.Unlock()

	seq, err := c.registerCall(call)
	if err != nil {
		call.mu.Lock()
		call.Error = err
		call.status = FINISHED
		call.mu.Unlock()
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
	call.mu.Lock()
	if call.status == FINISHED {
		call.mu.Unlock()
		return
	}
	call.status = RECEIVING
	call.mu.Unlock()

	if err := c.cc.WriteRequest(&req); err != nil {
		// 发送失败，直接返回call
		call := c.removeCall(seq)
		if call != nil {
			log.Printf("rpc client: sending error %s", err)
			call.mu.Lock()
			if call.status == FINISHED {
				call.mu.Unlock()
				return
			}
			call.Error = ErrShutdown
			call.status = FINISHED
			call.mu.Unlock()
			call.done()
		}
	}
}

// result must be a pointer
func (c *Client) Go(targetMethod string, result interface{}, args ...interface{}) *Call {
	if result != nil && reflect.ValueOf(result).Kind() != reflect.Ptr {
		panic("rpc client: result must be a pointer")
	}
	call := &Call{
		TargetMethod: targetMethod,
		Args:         args,
		Done:         make(chan struct{}),
		Reply:        result,
		status:       NEW,
		c:            c,
	}
	c.send(call)
	return call
}

func (c *Client) Call(targetMethod string, result interface{}, args ...interface{}) error {
	call := c.Go(targetMethod, result, args...)
	<-call.Done
	return call.Error
}

func (c *Client) CallUntil(duration time.Duration, targetMethod string, result interface{}, args ...interface{}) error {
	call := c.Go(targetMethod, result, args...)
	call.WaitFor(duration)
	return call.Error
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
		var resp protocol.ServerReply
		err = json.NewDecoder(conn).Decode(&resp)
		if err != nil {
			log.Printf("rpc client: %v", err)
			return
		}
		if resp.Code != "ok" {
			err = fmt.Errorf("server refused %s", resp)
			return
		}

		cc := codec.NewGobCodec(conn)
		c.cc = cc
		if resp.Tick != 0 {
			go c.ticker(resp.Tick)
		}

		return
	}()
	select {
	case <-time.After(timeOut):
		conn := <-connCh
		if conn != nil {
			closeCloseable(conn)
		}
		return ErrTimeOut
	case err := <-finish:
		if c.cc != nil && err != nil {
			closeCloseable(c.cc)
		}
		return err
	}
}

func (c *Client) ticker(tick int64) {
	for {
		err := c.cc.WriteRequest(&codec.Request{})
		if err != nil {
			log.Printf("rpc client: tick failed [%s], close connection", err)
			// 关闭连接
			closeCloseable(c)
			return
		}
		time.Sleep(time.Duration(tick) * time.Second)
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
