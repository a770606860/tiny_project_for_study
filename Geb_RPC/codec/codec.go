package codec

import (
	"bufio"
	"encoding/gob"
	"io"
	"reflect"
	"sync"
)

type Request struct {
	TargetMethod string
	Seq          uint64
	Argv         []interface{} // 请求包含的参数
}

type Response struct {
	Replyv interface{} // 返回的结果
	Seq    uint64
	Err    string // 服务端的错误
}

type Codec interface {
	io.Closer
	ReadResponse(response *Response) error
	ReadRequest(request *Request) error
	WriteRequest(request *Request) error
	WriteResponse(response *Response) error
	Failed() chan struct{}
	GetConn() io.ReadWriteCloser
}

type GobCodec struct {
	conn   io.ReadWriteCloser
	buf    *bufio.Writer
	dec    *gob.Decoder
	enc    *gob.Encoder
	failed chan struct{} // 数据发送队列
	types  sync.Map
}

func (c *GobCodec) Close() error {
	close(c.failed)
	return c.conn.Close()
}

func (c *GobCodec) Failed() chan struct{} {
	return c.failed
}

func (c *GobCodec) GetConn() io.ReadWriteCloser {
	return c.conn
}

func (c *GobCodec) ReadResponse(response *Response) error {
	c.register(response)
	return c.dec.Decode(response)
}

func (c *GobCodec) ReadRequest(request *Request) error {
	c.register(request)
	return c.dec.Decode(request)
}

func (c *GobCodec) WriteRequest(request *Request) (err error) {
	c.register(request)
	err = c.enc.Encode(request)
	if err != nil {
		return
	}
	return c.buf.Flush()
}

// Gob编解码器需要注册类型信息，否则编解码会出错
func (c *GobCodec) register(any interface{}) {
	if response, ok := any.(*Response); ok {
		if response.Replyv == nil {
			return
		}
		pkg := reflect.TypeOf(response.Replyv).PkgPath()
		if pkg == "" {
			return
		}
		name := pkg + "." + reflect.TypeOf(response.Replyv).Name()
		if _, loaded := c.types.LoadOrStore(name, nil); !loaded {
			gob.Register(response.Replyv)
		}
	} else {
		request := any.(*Request)
		for _, i := range request.Argv {
			pkg := reflect.TypeOf(i).PkgPath()
			name := pkg + "." + reflect.TypeOf(i).Name()
			if _, ok := c.types.LoadOrStore(name, nil); !ok {
				gob.Register(i)
			}
		}
	}
}

func (c *GobCodec) WriteResponse(response *Response) (err error) {
	c.register(response)
	err = c.enc.Encode(response)
	if err != nil {
		return
	}
	err = c.buf.Flush()
	return
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
