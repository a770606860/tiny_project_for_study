package client

import (
	"errors"
	"fmt"
	"gebrpc/protocol"
	"gebrpc/registry"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var mu sync.Mutex
var nameId int // 客户端名字生成器

type BalancedClient struct {
	// 注册中心，用以获取服务列表
	registry *registry.RegisterClient

	// 负载均衡策略：随机选取
	rand *rand.Rand

	option *protocol.Option

	mu      sync.Mutex
	clients map[string]*CClient // 服务
	closed  bool
}

type CClient struct {
	c    Client
	err  error
	down chan struct{}

	mu     sync.Mutex
	closed bool
}

func (c *CClient) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	return c.c.Close()
}

func (b *BalancedClient) Close() error {
	mu.Lock()
	if b.closed {
		mu.Unlock()
		return nil
	}
	b.closed = true
	reg := b.registry
	cli := b.clients
	b.registry = nil
	b.clients = nil
	b.rand = nil
	mu.Unlock()
	// 关闭资源
	closeCloseable(reg)
	for _, v := range cli {
		closeCloseable(v)
	}
	return nil
}

// must hold mu
func (b *BalancedClient) isClosed() bool {
	return b.closed
}

func (b *BalancedClient) IsClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.isClosed()
}

func (b *BalancedClient) Go(targetMethod string, result interface{}, args ...interface{}) *Call {
	names := strings.Split(targetMethod, ":")
	call := &Call{
		TargetMethod: targetMethod,
		Args:         args,
		Done:         make(chan struct{}),
		Reply:        result,
		status:       NEW,
	}
	if len(names) != 2 {
		call.Error = errors.New("illegal targetMethod name")
		call.status = FINISHED
		call.done()
		return call
	}

	b.mu.Lock()
	if b.isClosed() {
		b.mu.Unlock()
		call.Error = ErrShutdown
		call.status = FINISHED
		call.done()
		return call
	}
	ses, err := b.registry.GetServiceAdders(names[0])
	if err != nil {
		b.mu.Unlock()
		call.Error = err
		call.status = FINISHED
		call.done()
		return call
	}
	if len(ses) == 0 {
		b.mu.Unlock()
		call.Error = errors.New("no available clients instance")
		call.status = FINISHED
		call.done()
		return call
	}
	addr := ses[b.rand.Int()%len(ses)]
	c := b.clients[addr]
	// 如果客户端不存在则新建客户端
	// 对于同一个地址，同一时刻只允许一个客户端建立，其他对该地址的请求将会等待
	if c == nil {
		c = &CClient{down: make(chan struct{})}
		b.clients[addr] = c
		go func() {
			c.c, c.err = NewClient(addr, b.option)
			// 如果建立错误，则移除该客户端
			if c.err != nil {
				b.mu.Lock()
				if !b.isClosed() {
					delete(b.clients, addr)
				}
				b.mu.Unlock()
			}
			close(c.down)
		}()
	}
	b.mu.Unlock()
	// 等待客户端建立
	<-c.down
	if c.err != nil {
		call.Error = err
		call.status = FINISHED
		call.done()
		return call
	}
	// 调用方法
	return c.c.Go(targetMethod, result, args...)
}

func (b *BalancedClient) Call(targetMethod string, result interface{}, args ...interface{}) error {
	call := b.Go(targetMethod, result, args...)
	<-call.Done
	return call.Error
}

func (b *BalancedClient) CallUntil(duration time.Duration, targetMethod string, result interface{}, args ...interface{}) error {
	call := b.Go(targetMethod, result, args...)
	call.WaitFor(duration)
	return call.Error
}

func NewBalancedClient(addr, name, servAddr string, option *protocol.Option) (Client, error) {
	if len(name) == 0 {
		// 创建注册中心客户端
		mu.Lock()
		name = fmt.Sprintf("client-%d", nameId)
		nameId++
		mu.Unlock()
	}
	reg, err := registry.NewClient(name, servAddr, addr, time.Second*10)
	if err != nil {
		return nil, err
	}
	c := BalancedClient{registry: reg, option: option,
		clients: make(map[string]*CClient),
		rand:    rand.New(rand.NewSource(time.Now().UnixNano()))}
	return &c, nil
}
