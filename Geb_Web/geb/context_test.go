package geb

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestContext1(t *testing.T) {
	e := New()
	e.GET("/text", func(c *Context) {
		c.Text(http.StatusOK, "hello, welcome")
	})
	req := httptest.NewRequest("GET", "/text", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello, welcome", w.Body.String())
	e.GET("/json", func(c *Context) {
		c.JSON(http.StatusOK, map[string]string{
			"weiwei": "zheng",
			"gender": "male",
		})
	})
	req = httptest.NewRequest("GET", "/json", nil)
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	fmt.Println(w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, `{"gender":"male","weiwei":"zheng"}`+"\n", w.Body.String())
	e.GET("/data", func(c *Context) {
		c.Data(http.StatusOK, []byte("weiwei"))
	})
	req = httptest.NewRequest("GET", "/data", nil)
	w = httptest.NewRecorder()
	e.ServeHTTP(w, req)
	fmt.Println(w.Body.String())
	assert.Equal(t, http.StatusOK, w.Code)
	b := []byte("weiwei")
	for i, d := range w.Body.Bytes() {
		if !assert.Equal(t, b[i], d) {
			break
		}
	}
}

// 测试next和stop函数
func TestContext2(t *testing.T) {
	e := New()
	var out strings.Builder
	e.GET("/*", func(c *Context) {
		c.Data(http.StatusOK, nil)
	})
	e.POST("/*", func(c *Context) {
		c.Data(http.StatusOK, nil)
	})
	e.AddMiddlewire("/", func(c *Context) {
		out.WriteString("a")
		c.Next()
		out.WriteString("z")
	})
	e.AddMiddlewire("/next", func(c *Context) {
		out.WriteString("b")
	})
	req := httptest.NewRequest("GET", "/next", nil)
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abz", out.String())
	e.AddMiddlewire("/get", func(c *Context) {
		// 通过
		if c.Req.Method == "GET" {
			out.WriteString("d")
		} else {
			c.Fail(500)
		}
	})
	req = httptest.NewRequest("GET", "/get", nil)
	w = httptest.NewRecorder()
	out.Reset()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "adz", out.String())

	req = httptest.NewRequest("POST", "/get", nil)
	w = httptest.NewRecorder()
	out.Reset()
	e.ServeHTTP(w, req)
	assert.Equal(t, 500, w.Code)
	assert.Equal(t, "az", out.String())
}
