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
	Done         chan struct{} // 通知调用方调用完成，当Done被关闭时表明完成
	c            *Client
	mu           sync.Mutex
	status       int
}

// call status
const (
	NEW = iota
	RECEIVING
	FINISHED
)

var ErrTimeOut = errors.New("connect or negotiate server time out")
var ErrWaitingForReceiving = errors.New("call canceled while sending or waiting-for-receiving")
var ErrShutdown = errors.New("connection is shut down and call didn't send")

func (call *Call) done() {
	close(call.Done)
}

// TODO：后续可以尝试添加单向关闭功能
// 不变式：pending中保存的call都处于RECEIVING，等待处理结果状态
type Client struct {
	sending sync.Mutex // 发送锁，因为只有一个连接因此串行发送操作
	mu      sync.Mutex // mu保护下面所有的字段
	cc      codec.Codec
	pending map[uint64]*Call // 正在进行中的call
	seq     uint64
	closed  bool
}

var _ io.Closer = (*Client)(nil)

// 等待指定时间后返回，如果call未完成则关闭call
func (call *Call) WaitFor(duration time.Duration) {
	select {
	case <-time.After(duration): // 超时
		call := call.c.removeCall(call.Seq)
		if call == nil {
			return
		}
		call.mu.Lock()
		call.Error = ErrWaitingForReceiving
		call.status = FINISHED
		call.mu.Unlock()
		call.done()
	case <-call.Done: // call完成
	}
}

func (c *Client) Close() error {
	c.mu.Lock()
	if c.isClosed() {
		c.mu.Unlock()
		return nil
	}
	c.terminateCalls()
	cc := c.cc
	c.pending = nil
	c.closed = true
	c.mu.Unlock()
	if cc == nil {
		return nil
	}
	return cc.Close()
}

// must hold mu
func (c *Client) isClosed() bool {
	return c.closed
}

func (c *Client) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.isClosed()
}

func (c *Client) registerCall(call *Call) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.isClosed() {
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
	if c.pending == nil {
		return nil
	}
	call, ok := c.pending[seq]
	if ok {
		delete(c.pending, seq)
	}
	return call
}

// must hold mu before call this func
// assert c.isClosed() false
func (c *Client) terminateCalls() {
	for _, call := range c.pending {
		call.mu.Lock()
		call.Error = ErrWaitingForReceiving
		call.status = FINISHED
		call.mu.Unlock()
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
		// Json解码器需要将Request拆开为head, body进行发送，从head获取类型信息后再利用这些信息解码body；
		if err = c.cc.ReadResponse(&resp); err != nil {
			log.Printf("rpc client: read error [%s]", err)
			break
		}
		call := c.removeCall(resp.Seq)
		// 确保call未完成
		if call != nil {
			call.mu.Lock()
			if call.status != RECEIVING {
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

	// 接收通道损坏，关闭客户端
	closeCloseable(c)
}

func (c *Client) send(call *Call) {
	seq, err := c.registerCall(call)
	if err != nil {
		call.mu.Lock()
		call.Error = err
		call.status = FINISHED
		call.mu.Unlock()
		call.done()
		return
	}

	c.sending.Lock()
	defer c.sending.Unlock()

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
	// 改变状态前确认请求未完成
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
			call.Error = ErrShutdown
			call.status = FINISHED
			call.mu.Unlock()
			call.done()
		}

		// 发送通道损坏，关闭客户端
		closeCloseable(c)
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

func (c *Client) negotiate(conn net.Conn, option *protocol.Option) error {
	err := json.NewEncoder(conn).Encode(option)
	log.Println("rpc client: option sent")
	if err != nil {
		// log.Printf("rpc client: %v", err)
		return err
	}
	var resp protocol.ServerReply
	err = json.NewDecoder(conn).Decode(&resp)
	if err != nil {
		// log.Printf("rpc client: %v", err)
		return err
	}
	if resp.Code != "ok" {
		err = fmt.Errorf("server refused %s", resp.Code)
		return err
	}

	c.mu.Lock()
	if !c.isClosed() {
		cc := codec.NewGobCodec(conn)
		c.cc = cc
	}
	c.mu.Unlock()

	if resp.Tick != 0 {
		go c.ticker(resp.Tick)
	}
	return nil
}

func (c *Client) dail(addr string, option *protocol.Option) error {
	conn, err := net.Dial("tcp", addr)
	if err == nil {
		log.Printf("rpc client: connection established, negociating")
	} else {
		return err
	}
	err = c.negotiate(conn, option)
	if err != nil {
		closeCloseable(c)
	}
	return err
}

func (c *Client) dailTimeout(addr string, timeOut time.Duration, option *protocol.Option) error {
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
		err = c.negotiate(conn, option)
	}()
	select {
	case <-time.After(timeOut):
		// 等待连接返回，释放连接资源
		<-connCh
		closeCloseable(c)

		return ErrTimeOut
	case err := <-finish:
		if err != nil {
			// 建立连接或协商错误，关闭客户端
			closeCloseable(c)
		}
		return err
	}
}

func (c *Client) ticker(tick int64) {
	for {
		err := c.cc.WriteRequest(&codec.Request{})
		if err != nil {
			log.Printf("rpc client: tick failed [%s], close connection", err)
			// 心跳失败，关闭客户端
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

func NewClientTimeOut(addr string, timeOut time.Duration, option *protocol.Option) (*Client, error) {
	c := &Client{pending: make(map[uint64]*Call)}
	op := parseOption(option)
	if err := c.dailTimeout(addr, timeOut, op); err != nil {
		return nil, err
	}
	go c.receive()
	return c, nil
}

func NewClient(addr string, option *protocol.Option) (*Client, error) {
	c := &Client{pending: make(map[uint64]*Call)}
	op := parseOption(option)
	if err := c.dail(addr, op); err != nil {
		return nil, err
	}
	go c.receive()
	return c, nil
}

func parseOption(option *protocol.Option) *protocol.Option {
	if option == nil {
		return protocol.DefaultOption
	}
	return option
}
