package registry

import (
	"bytes"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Service struct {
	name    _name         // 服务名
	addr    _addr         // 服务地址
	id      _id           // 服务唯一id
	tick    time.Duration // 心跳
	aliveCh chan struct{}

	lAddr _IPv4 // 服务监听地址，用于接收注册中心的通知，_IPv4

	mu       sync.Mutex
	closed   bool
	reg      *RegisterServer
	interest map[_name]struct{} // 该服务关心的服务集合
}

func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	if s.aliveCh != nil {
		close(s.aliveCh)
	}
	s.mu.Unlock()
	return nil
}

type RegisterServer struct {
	seMu        sync.Mutex
	services    map[_name][]*Service // 可用服务集合
	idToService map[_id]*Service
	id          _id // id生成

	clMu    sync.Mutex
	clients map[_id]*Service // 注册了被动更新的客户端集合

	mu     sync.Mutex
	addr   _IPv4
	closed bool
	l      net.Listener
}

func (reg *RegisterServer) Close() error {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	if !reg.closed {
		reg.closed = true
		if reg.l != nil {
			return reg.l.Close()
		}
	}
	return nil
}

func (reg *RegisterServer) Addr() string {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	return reg.addr
}

// 注册成功则返回
func (reg *RegisterServer) register(name _name, addr _addr, lAddr _IPv4, tick time.Duration) (*Service, error) {
	se := Service{name: name, addr: addr, lAddr: lAddr, tick: tick,
		reg: reg, interest: make(map[_name]struct{})}
	// 启动心跳监测
	err := reg.issueAliveCheck(&se)
	if err != nil {
		return nil, err
	}

	reg.seMu.Lock()
	reg.id = reg.id + 1
	se.id = reg.id
	reg.services[name] = append(reg.services[name], &se)
	reg.idToService[se.id] = &se
	reg.seMu.Unlock()
	if len(lAddr) != 0 {
		reg.clMu.Lock()
		reg.clients[se.id] = &se
		reg.clMu.Unlock()
	}
	// 推送更新
	go reg.informChanges(&se)
	log.Printf("rpc registry: Service %d registed", reg.id)
	return &se, nil
}

func (reg *RegisterServer) resign(id _id) {
	reg.seMu.Lock()
	s := reg.idToService[id]
	if s == nil {
		reg.seMu.Unlock()
		return
	}
	ses := reg.services[s.name]
	// 从列表中删除服务
	delete(reg.idToService, s.id)
	newSes := make([]*Service, 0, len(ses)-1)
	for _, ss := range ses {
		if ss.id == s.id {
			continue
		}
		newSes = append(newSes, ss)
	}
	if len(newSes) == 0 {
		delete(reg.services, s.name)
	} else {
		reg.services[s.name] = newSes
	}
	reg.seMu.Unlock()
	if len(s.lAddr) != 0 {
		reg.clMu.Lock()
		delete(reg.clients, s.id)
		reg.clMu.Unlock()
	}
	// 推送更新
	reg.informChanges(s)
	// 关闭服务
	err := s.Close()
	if err != nil {
		log.Printf("rpc registry: close Service %d failed, %v", s.id, err)
	}
	log.Printf("rpc registry: Service %d removed", s.id)
}

func (reg *RegisterServer) getServiceAddr(id _id, name _name) []_addr {
	reg.seMu.Lock()
	s := reg.idToService[id]
	sers := reg.services[name]
	if s == nil {
		reg.seMu.Unlock()
		return nil
	}
	// 将服务添加到感兴趣集合中
	s.mu.Lock()
	s.interest[name] = struct{}{}
	s.mu.Unlock()

	addrs := make([]_addr, 0, len(sers))
	for _, s := range sers {
		addrs = append(addrs, s.addr)
	}
	reg.seMu.Unlock()

	return addrs
}

func (reg *RegisterServer) issueAliveCheck(s *Service) error {
	s.aliveCh = make(chan struct{}, 1)
	tick := int(s.tick.Seconds() * 3)
	if tick == 0 {
		return errors.New("tick time must bigger than 1s")
	}
	go func() {
		for {
			select {
			case <-time.After(s.tick * 3):
				log.Printf("rpc registry: tick failed, Service Id %d", s.id)
				reg.resign(s.id)
				return
			case _, ok := <-s.aliveCh:
				if !ok {
					return
				}
			}
		}

	}()
	return nil
}

func (reg *RegisterServer) informChanges(s *Service) {
	var toBeInformed []*Service
	reg.clMu.Lock()
	for _, v := range reg.clients {
		v.mu.Lock()
		if _, ok := v.interest[s.name]; ok {
			toBeInformed = append(toBeInformed, v)
		}
		v.mu.Unlock()
	}
	reg.clMu.Unlock()
	if len(toBeInformed) == 0 {
		return
	}
	var updates []_addr
	reg.seMu.Lock()
	updates = make([]_addr, 0, len(reg.services[s.name]))
	for _, ss := range reg.services[s.name] {
		updates = append(updates, ss.addr)
	}
	reg.seMu.Unlock()
	for _, ss := range toBeInformed {
		go reg.sendUpDateHTTP(ss, s.name, updates)
	}
}

// addr可以为nil，此时该服务很可能不提供服务，只索取服务
func (reg *RegisterServer) HandleRegister(w http.ResponseWriter, r *http.Request) {
	name := r.Header.Get("name")
	addr := r.Header.Get("addr")
	tick, err := strconv.Atoi(r.Header.Get("tick"))
	lAddr := r.Header.Get("lAddr")
	if len(name) == 0 || tick == 0 || err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	s, err := reg.register(name, addr, lAddr, time.Second*time.Duration(tick))
	if err != nil {
		log.Printf("rpc registry: register error %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("id", strconv.Itoa(s.id))
	return
}

func (reg *RegisterServer) HandleHeartBeat(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.Header.Get("id"))
	if err != nil {
		log.Printf("rpc register: heartbeat no Id")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// log.Printf("rpc registry: heartbeat client Id=%d", Id)

	reg.seMu.Lock()
	s := reg.idToService[id]
	reg.seMu.Unlock()

	if s == nil {
		// log.Printf("rpc registry: heartbeat no Id=%d ", Id)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		// log.Printf("rpc registry: heartbeat Id=%d has closed", Id)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// 写通道时必须持有锁否则可能导致向关闭通道写入数据的异常
	select {
	case s.aliveCh <- struct{}{}:
	default:
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (reg *RegisterServer) HandleResign(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.Header.Get("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	reg.resign(id)
	w.WriteHeader(http.StatusOK)
}

func (reg *RegisterServer) HandleGetServiceAddr(w http.ResponseWriter, r *http.Request) {
	name := r.Header.Get("name")
	id, err := strconv.Atoi(r.Header.Get("id"))

	if len(name) == 0 || err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	addr := reg.getServiceAddr(id, name)
	data, err := json.Marshal(addr)
	if err != nil {
		log.Printf("rpc registry: get service error %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(data)
	if err != nil {
		log.Printf("rpc registry: send replay error %v", err)
	}
}

// 使用JSON编码，发送Http更新
func (reg *RegisterServer) sendUpDateHTTP(s *Service, name _name, addrs []_addr) {
	// TODO 验证nil或空interest的编码结果
	data, err := json.Marshal(addrs)
	if err != nil {
		log.Printf("rpc registry: send update error %v", err)
		return
	}
	url := getHttpURL(s.lAddr, "/update")
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		log.Printf("rpc registry: send update error %v", err)
		return
	}
	req.Header.Set("name", name)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("rpc registry: send update error %v", err)
		return
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("rpc registry: send update failed, code %d", resp.StatusCode)
	}
}

func StartServer() (*RegisterServer, error) {
	se := &RegisterServer{services: make(map[_name][]*Service),
		idToService: make(map[_id]*Service),
		clients:     make(map[_id]*Service)}
	ser := http.NewServeMux()
	ser.HandleFunc("/heartbeat", se.HandleHeartBeat)
	ser.HandleFunc("/services", se.HandleGetServiceAddr)
	ser.HandleFunc("/resign", se.HandleResign)
	ser.HandleFunc("/register", se.HandleRegister)
	l, err := net.Listen("tcp4", "")
	if err != nil {
		return nil, err
	}
	se.addr = l.Addr().String()
	go func() {
		// 启动服务器
		_ = http.Serve(l, ser)
	}()
	se.l = l
	return se, nil
}

// 用于查看测试信息
func (reg *RegisterServer) PrintInfo() {
	reg.seMu.Lock()
	for _, v := range reg.services {
		for _, s := range v {
			log.Printf("%s Id=%v, Addr=%s, interest=%v", s.name, s.id, s.addr, s.interest)
		}
	}
	reg.seMu.Unlock()
	reg.clMu.Lock()
	for _, v := range reg.clients {
		log.Printf("%s Id=%v, lAddr=%s", v.name, v.id, v.lAddr)
	}
	reg.clMu.Unlock()
}
