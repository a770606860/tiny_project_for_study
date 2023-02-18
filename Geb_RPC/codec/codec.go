package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"log"
)

type Request struct {
	TargetMethod string
	Seq          uint64
	Argv         []interface{} // 请求包含的参数
}

type Response struct {
	Replyv interface{} // 返回的结果
	Seq    uint64
	Err    string // 服务端的错误，因为有些编码不支持error类型，所以这里使用string
}

type Codec interface {
	io.Closer
	Read(any interface{}) error
	Write(any interface{}) error
	Failed() chan struct{}
}

type GobCodec struct {
	conn   io.ReadWriteCloser
	buf    *bufio.Writer
	dec    *gob.Decoder
	enc    *gob.Encoder
	failed chan struct{} // 数据发送队列
}

func (c *GobCodec) Close() error {
	close(c.failed)
	return c.conn.Close()
}

func (c *GobCodec) Failed() chan struct{} {
	return c.failed
}

func (c *GobCodec) Read(any interface{}) error {
	return c.dec.Decode(any)
}

func (c *GobCodec) Write(any interface{}) (err error) {
	defer func() {
		e := c.buf.Flush()
		if e != nil {
			log.Printf("rpc codec: buf flush err %s", e)
		}
	}()
	err = c.enc.Encode(any)
	return
}

func (c *GobCodec) WriteResponse(resp *Response) (err error) {
	err = c.write(resp)
	if err != nil {
		log.Printf("rpc codec: gob error encoding response[Seq:err]:[%d,%s]",
			resp.Seq, err)
	}
	return err
}

func (c *GobCodec) write(any interface{}) (err error) {
	defer func() {
		_ = c.buf.Flush()
		if err != nil {
			_ = c.Close()
		}
	}()
	return c.enc.Encode(any)
}

func NewGobCodec(conn io.ReadWriteCloser) Codec {
	buf := bufio.NewWriter(conn)
	return &GobCodec{
		conn:   conn,
		buf:    buf,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(buf),
		failed: make(chan struct{}),
	}
}

type Type int

const (
	GobType = iota
	JsonType
)
