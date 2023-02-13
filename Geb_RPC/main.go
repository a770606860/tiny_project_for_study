package main

import (
	"encoding/json"
	"gebrpc/codec"
	"gebrpc/protocol"
	"gebrpc/server"
	"io"
	"log"
	"net"
	"time"
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
	conn, err := net.Dial("tcp", <-addr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = conn.Close()
	}()
	_ = json.NewEncoder(conn).Encode(protocol.DefaultOption)
	time.Sleep(200 * time.Millisecond) // 防止服务端json解码影响gob解码
	cc := codec.NewGobCodec(conn)
	err = cc.Write(&codec.Request{TargetMethod: "foo", Seq: 1, Argv: "xiaobai"})
	if err != nil {
		log.Println(err)
	}
	err = cc.Write(&codec.Request{TargetMethod: "bao", Seq: 2, Argv: "xiaonai"})
	if err != nil {
		log.Println(err)
	}
	var s1, s2 codec.Response
	err = cc.Read(&s1)
	if err != nil {
		log.Println(err)
	}
	log.Println(s1.Replyv)
	err = cc.Read(&s2)
	if err != nil {
		log.Println(err)
	}
	log.Println(s2.Replyv)
}
