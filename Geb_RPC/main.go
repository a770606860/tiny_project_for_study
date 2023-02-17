package main

import (
	"gebrpc/client"
	"gebrpc/server"
	"io"
	"log"
	"net"
)

func startServer(addr chan string) {
	l, err := net.Listen("tcp", ":0")
	var conn io.ReadWriteCloser
	if err != nil {
		log.Fatal("start server error ", err)
	}
	log.Println("start rpc server on ", l.Addr().String())
	addr <- l.Addr().String()
	conn, err = l.Accept()
	if err != nil {
		log.Fatal(err)
	}
	server.NewServer().ServeConn(conn)
}

func main() {
	addr := make(chan string)
	go startServer(addr)
	c, err := client.NewClient(<-addr)
	if err != nil {
		log.Fatal("New client error: ", err)
	}
	var r1, r2 string
	call1 := c.Go("foo", "xiaobai", &r1)
	call2 := c.Go("bar", "xiaoju", &r2)
	print1 := func(call *client.Call) {
		if call.Error != nil {
			log.Printf("%s:%s:%d error %s", call.TargetMethod,
				call.Args, call.Seq, call.Error)
		} else {
			log.Printf("%s returned", call.Reply.(string))
		}
	}
	for i := 0; i < 2; i++ {
		select {
		case <-call1.Done:
			print1(call1)
		case <-call2.Done:
			print1(call2)
		}
	}
}
