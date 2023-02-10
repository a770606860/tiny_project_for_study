package geb

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Context 上下文
type Context struct {
	// 原始数据
	Writer http.ResponseWriter
	Req    *http.Request

	engine *Engine

	handlers []HandlerFunc
	i        int
	stoped   bool
}

// 等价于c.Writer.Write([]byte)
func (c *Context) Write(data []byte) (int, error) {
	return c.Writer.Write(data)
}

// SetHeader 等价于c.Writer.Header().Set(string,string)
func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

// SetStatus 等价于c.Writer.WriteHeader(int)
func (c *Context) SetStatus(status int) {
	c.Writer.WriteHeader(status)
}

func (c *Context) SetContentType(t string) {
	c.SetHeader("Content-Type", t)
}

func (c *Context) Text(status int, str string) (int, error) {
	c.SetContentType("text/plain")
	c.SetStatus(status)
	return c.Write([]byte(str))
}

func (c *Context) JSON(status int, obj interface{}) error {
	c.SetContentType("application/json")
	c.SetStatus(status)
	encoder := json.NewEncoder(c.Writer)
	return encoder.Encode(obj)
}

func (c *Context) Data(status int, data []byte) (int, error) {
	c.SetStatus(status)
	return c.Write(data)
}

// todo
func (c *Context) HTML(status int, source string, data interface{}) error {
	return fmt.Errorf("暂时还未实现，%s", "嘿嘿")
	//c.SetContentType("text/html")
	//c.SetStatus(status)
	//return c.engine.htmlTemplate.ExecuteTemplate(c.Writer, source, data)
}

func (c *Context) Fail(code int) {
	c.Data(code, nil)
	c.i = len(c.handlers)
}

func (c *Context) run() {
	for c.i < len(c.handlers) {
		c.handlers[c.i](c)
		c.i++
	}
}

func (c *Context) Next() {
	c.i++
	c.run()
}

func newCtx(w http.ResponseWriter, req *http.Request, handlers []HandlerFunc) *Context {
	return &Context{Writer: w, Req: req, handlers: handlers}
}
