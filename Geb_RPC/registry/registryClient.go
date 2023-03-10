package registry

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// 和注册中心的通信使用http协议，JSON编码
type RegisterClient struct {
	mu sync.Mutex
	// 注册中心地址，IPv4地址
	serverAddr _IPv4

	// 自身服务名地址与ID
	name _name
	id   _id
	addr _addr

	tick time.Duration

	// 监听地址，接收自动更新
	lAddr _IPv4 // _IPv4
	l     net.Listener
	ch    chan Updates

	// 服务及其地址
	services map[_name][]_addr
	closed   bool
}

func (reg *RegisterClient) SetName(name string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.name = name
}

func (reg *RegisterClient) Id() int {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.id
}

func (reg *RegisterClient) SetId(id int) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.id = id
}

func (reg *RegisterClient) Addr() string {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.addr
}

func (reg *RegisterClient) SetAddr(addr string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.addr = addr
}

func (reg *RegisterClient) Tick() time.Duration {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.tick
}

func (reg *RegisterClient) SetTick(tick time.Duration) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.tick = tick
}

func (reg *RegisterClient) LAddr() string {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.lAddr
}

func (reg *RegisterClient) SetLAddr(lAddr string) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.lAddr = lAddr
}

func (reg *RegisterClient) L() net.Listener {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.l
}

func (reg *RegisterClient) SetL(l net.Listener) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.l = l
}

func (reg *RegisterClient) ServerAddr() string {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.serverAddr
}

func (reg *RegisterClient) Name() string {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.name
}

type Updates struct {
	name  _name
	addrs []_addr
}

var ErrClosed = errors.New("client already closed")

func (reg *RegisterClient) GetServiceAdders(name string) ([]string, error) {
	if len(name) == 0 {
		return nil, errors.New("Name must not empty")
	}
	reg.mu.Lock()
	if reg.closed {
		reg.mu.Unlock()
		return nil, ErrClosed
	}
	s, ok := reg.services[name]
	if !ok {
		reg.mu.Unlock()
		return reg.GetServiceAddrsForce(name)
	} else {
		addr := make([]string, len(s))
		copy(addr, s)
		reg.mu.Unlock()
		return addr, nil
	}
}

func (reg *RegisterClient) GetServiceAddrsForce(name string) ([]string, error) {
	if len(name) == 0 {
		return nil, errors.New("Name must not empty")
	}
	addrs, err := reg.getAddrsHTTP(name)
	if err != nil {
		return nil, err
	}
	reg.mu.Lock()
	reg.services[name] = addrs
	reg.mu.Unlock()
	return addrs, nil
}

func (reg *RegisterClient) register(name, addr string, tick time.Duration, update bool) error {
	if int(tick.Seconds()) <= 0 {
		return errors.New("tick must larger than 1s")
	}
	if update {
		l, err := net.Listen("tcp4", "")
		if err != nil {
			return err
		}
		reg.SetLAddr(l.Addr().String())
		reg.SetL(l)
		reg.ch = make(chan Updates, 10)
		// 启动监听服务
		go func() {
			handler := http.NewServeMux()
			handler.HandleFunc("/update", reg.HandleUpdate)
			_ = http.Serve(l, handler)
		}()
		go reg.doUpdate()
	}
	if err := reg.registerHTTP(name, addr, tick, update); err != nil {
		if update {
			if err := reg.L().Close(); err != nil {
				log.Printf("rpc registry: stop lAddr %s error %v", reg.LAddr(), err)
			}
			close(reg.ch)
			reg.mu.Lock()
			reg.l = nil
			reg.ch = nil
			reg.lAddr = ""
			reg.mu.Unlock()
		}
		return err
	}
	return nil
}

func (reg *RegisterClient) resign() error {
	return reg.resignHTTP()
}

func (reg *RegisterClient) Close() error {
	reg.mu.Lock()
	if reg.closed {
		reg.mu.Unlock()
		return nil
	}
	reg.closed = true
	reg.mu.Unlock()
	close(reg.ch)
	err := reg.resign()
	if reg.L() != nil {
		if err := reg.l.Close(); err != nil {
			log.Printf("rpc registry: close lAddr error %v", err)
		}
	}
	return err
	// TODO 赋空字段
}

func (reg *RegisterClient) update(name _name, addrs []_addr) {
	reg.mu.Lock()
	if len(addrs) == 0 {
		delete(reg.services, name)
	} else {
		reg.services[name] = addrs
	}
	reg.mu.Unlock()
}

func (reg *RegisterClient) doUpdate() {
	for v := range reg.ch {
		reg.update(v.name, v.addrs)
	}
}

func (reg *RegisterClient) heartBeat() {
	for {
		time.Sleep(reg.Tick())
		reg.mu.Lock()
		if reg.closed {
			reg.mu.Unlock()
			return
		}
		reg.mu.Unlock()
		go func() {
			err := reg.heartBeatHTTP()
			if err != nil {
				// log.Printf("rpc registry: heartbeat Id=%d error %v", reg.Id, err)
			}
		}()
	}
}

func (reg *RegisterClient) HandleUpdate(writer http.ResponseWriter, request *http.Request) {
	name := request.Header.Get("name")
	var addrs []_IPv4
	data, _ := ioutil.ReadAll(request.Body)
	_ = request.Body.Close()
	err := json.Unmarshal(data, &addrs)
	if err != nil {
		log.Printf("rpc registry: HandleUpdate failed %v", err)
		writer.WriteHeader(http.StatusOK)
		return
	}
	reg.mu.Lock()
	if !reg.closed {
		update := Updates{name: name, addrs: addrs}
		// 推送更新
		reg.ch <- update
	}
	reg.mu.Unlock()
	writer.WriteHeader(http.StatusOK)
}

func (reg *RegisterClient) getAddrsHTTP(name _name) ([]string, error) {
	url := getHttpURL(reg.ServerAddr(), "/services")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("name", name)
	req.Header.Set("id", strconv.Itoa(reg.Id()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	var addrs []_IPv4
	data, _ := ioutil.ReadAll(resp.Body)
	err = json.Unmarshal(data, &addrs)
	if err != nil {
		return nil, err
	}
	return addrs, nil
}

// 注册名为name的服务，并定期向服务端发送心跳，注册中心在三倍心跳期内未接收到心跳则移除该服务
// 如果update为true将会启动一个监听地址监听服务端更新
// 如果发生错误，那么需要关闭
func (reg *RegisterClient) registerHTTP(name _name, addr _addr, tick time.Duration, update bool) error {
	url := getHttpURL(reg.ServerAddr(), "/register")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("name", name)
	req.Header.Set("addr", addr)
	req.Header.Set("tick", strconv.Itoa(int(tick.Seconds())))
	if update {
		req.Header.Set("lAddr", reg.LAddr())
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("registry failed, code %d", resp.StatusCode)
	}
	// 获取自身ID
	id, err := strconv.Atoi(resp.Header.Get("id"))
	if err != nil || id <= 0 {
		return errors.New("error no Id")
	}
	reg.SetId(id)
	return nil
}

// 注销
func (reg *RegisterClient) resignHTTP() error {
	url := getHttpURL(reg.ServerAddr(), "/resign")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("id", strconv.Itoa(reg.Id()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resign client failed, code %d", resp.StatusCode)
	}
	return nil
}

func (reg *RegisterClient) heartBeatHTTP() error {
	url := getHttpURL(reg.ServerAddr(), "/heartbeat")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("id", strconv.Itoa(reg.Id()))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("response code %d", resp.StatusCode)
	}
	return nil
}

func NewClient(name, addr, serverAddr string, tick time.Duration) (*RegisterClient, error) {
	c := &RegisterClient{name: name, addr: addr, serverAddr: serverAddr, tick: tick}
	c.services = make(map[_name][]_addr)
	err := c.register(name, addr, tick, true)
	if err != nil {
		return nil, err
	}
	// 启动心跳
	go c.heartBeat()
	return c, nil
}
