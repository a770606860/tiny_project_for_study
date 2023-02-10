package geb

import (
	"reflect"
	"testing"
)

func TestIsValidPattern(t *testing.T) {
	if !isValidPatten("/") {
		t.Error("/ should valid")
	}
	if !isValidPatten("/a/b/c") {
		t.Error("/a/b/c should valid")
	}
	if !isValidPatten("/a/") {
		t.Error("/a/ should valid")
	}
	if !isValidPatten("/./c") {
		t.Error("/./c should valid")
	}
	if !isValidPatten("/a/./c") {
		t.Error("/a/./c should valid")
	}
	if !isValidPatten("/*") {
		t.Error("/* should valid")
	}
	if !isValidPatten("/*/") {
		t.Error("/*/ should valid")
	}
	if !isValidPatten("/a/*") {
		t.Error("/a/* should valid")
	}

	if isValidPatten("a") || isValidPatten("a/b/") {
		t.Error(`must start with "/"`)
	}

	if isValidPatten("/a/*/b") {
		t.Error("/a/*/b should invalid")
	}
	if isValidPatten("/*/a") {
		t.Error("/*/a should invalid")
	}
	if isValidPatten("/*/*") {
		t.Error("/*/* should invalid")
	}
	if isValidPatten("/a/*/c/*") {
		t.Error("/a/*/c/* should invalid")
	}
	if isValidPatten("/a/*/*") {
		t.Error("/a/*/* should invalid")
	}
	if isValidPatten("/a/*/*/") {
		t.Error("/a/*/*/ should invalid")
	}

	if isValidPatten("/a/.") {
		t.Error("/a/. should invalid")
	}
	if isValidPatten("/a/./") {
		t.Error("/a/./ should invalid")
	}
	if isValidPatten("/.") {
		t.Error("/. should invalid")
	}
	if isValidPatten("/./") {
		t.Error("/./ should invalid")
	}
	if isValidPatten("/./.") {
		t.Error("/./. should invalid")
	}
	if isValidPatten("/a/././b") {
		t.Error("/a/././b should invalid")
	}

	if isValidPatten("/a/./*") {
		t.Error("/a/./* should invalid")
	}
	if isValidPatten("/a/*/.") {
		t.Error("/a/*/. should invalid")
	}
	if isValidPatten("/././*/a") {
		t.Error("/././*/a should invalid")
	}
}

func TestMethodToInt(t *testing.T) {
	if methodToInt("GET") != 0 {
		t.Error("GET should be 0")
	}
	if methodToInt("POST") != 1 {
		t.Error("POST should be 1")
	}
	if methodToInt("PATCH") != -1 {
		t.Error("PATCH should be -1")
	}
}

func TestSplitP(t *testing.T) {
	s := "/"
	sp := splitP(s)
	if !reflect.DeepEqual(sp, []string{"/"}) {
		t.Error(s + " split error")
	}
	s = ""
	sp = splitP(s)
	if len(sp) != 0 {
		t.Error(s + " split error")
	}
	s = "/a/"
	sp = splitP(s)
	if !reflect.DeepEqual(sp, []string{"/", "a"}) {
		t.Error(s + " split error")
	}
	s = "/a"
	sp = splitP(s)
	if !reflect.DeepEqual(sp, []string{"/", "a"}) {
		t.Error(s + " split error")
	}
	s = "/a/b"
	sp = splitP(s)
	if !reflect.DeepEqual(sp, []string{"/", "a", "b"}) {
		t.Error(s + " split error")
	}
	s = "/a/b/"
	sp = splitP(s)
	if !reflect.DeepEqual(sp, []string{"/", "a", "b"}) {
		t.Error(s + " split error")
	}
}

func TestRouter(t *testing.T) {
	router := defaultRouter{}
	handler := router.Handlers("GET", "/")
	if len(handler) != 0 {
		t.Error("1")
	}
	// handlers
	// GET:
	// /a/b/* f1
	// /a/b/c f2
	// /a/c	  f3
	// /a/./c f4
	// /b	  f7
	// POST
	// /a/b/c f5
	// PATCH
	// /	  f2
	// ---------------
	// midllerwires
	// /	  m1
	// /a	  m2
	// /a/b/c/d m3, m4
	f1 := func(c *Context) {}
	f2 := func(c *Context) {}
	f3 := func(c *Context) {}
	f4 := func(c *Context) {}
	f5 := func(c *Context) {}
	f6 := func(c *Context) {}
	f7 := func(c *Context) {}
	m1 := func(c *Context) {}
	m2 := func(c *Context) {}
	m3 := func(c *Context) {}
	m4 := func(c *Context) {}
	router.AddHandler("GET", "/a/b/*", f1)
	router.AddHandler("GET", "/a/b/c", f2)
	router.AddHandler("GET", "/a/c", f3)
	router.AddHandler("GET", "/a/./c", f4)
	router.AddHandler("GET", "/b", f7)
	router.AddHandler("POST", "/a/b/c", f5)
	router.AddHandler("POST", "/a/b/c/d", f6)
	router.AddMiddlewire("/", m1)
	router.AddMiddlewire("/a", m2)
	router.AddMiddlewire("/a/b/c/d", m3, m4)
	n := countFunc(router.root)
	if n != 11 {
		t.Errorf("n is %d except 11\n", n)
	}
	// get /a \
	h := router.Handlers("GET", "/a")
	if len(h) != 0 {
		t.Error("1")
	}
	// get /a/b \
	h = router.Handlers("GET", "/a/b")
	if len(h) != 0 {
		t.Error("2")
	}
	// get /a/b/c m1, m2, f2
	h = router.Handlers("GET", "/a/b/c")
	q := []HandlerFunc{m1, m2, f2}
	if !isFunEqual(h, q) {
		t.Error("3")
	}
	// get /a/b/c/d m1, m2, m3, m4, f1
	h = router.Handlers("GET", "/a/b/c/d")
	q = []HandlerFunc{m1, m2, m3, m4, f1}
	if !isFunEqual(h, q) {
		t.Error("4")
	}
	// get /a/c m1, m2, f3
	h = router.Handlers("GET", "/a/c")
	q = []HandlerFunc{m1, m2, f3}
	if !isFunEqual(h, q) {
		t.Error("5")
	}
	// get /b m1, f7
	h = router.Handlers("GET", "/b")
	q = []HandlerFunc{m1, f7}
	if !isFunEqual(h, q) {
		t.Error("6")
	}
	// post / \
	h = router.Handlers("POST", "/")
	if len(h) != 0 {
		t.Error("7")
	}
	// post /a/b/c m1, m2, f5
	h = router.Handlers("POST", "/a/b/c")
	q = []HandlerFunc{m1, m2, f5}
	if !isFunEqual(h, q) {
		t.Error("8")
	}
	// patch / \
	h = router.Handlers("PATCH", "/")
	if len(h) != 0 {
		t.Error("9")
	}

}

func isFunEqual(h []HandlerFunc, q []HandlerFunc) bool {
	if len(h) != len(q) {
		return false
	}
	for i, f := range h {
		v1 := reflect.ValueOf(f)
		v2 := reflect.ValueOf(q[i])
		if v1.Pointer() != v2.Pointer() {
			return false
		}
	}
	return true
}

func countFunc(n *node) int {
	if n == nil {
		return 0
	}
	c := len(n.midwirs)
	for _, f := range n.handlers {
		if f != nil {
			c++
		}
	}
	for _, nn := range n.children {
		c += countFunc(nn)
	}
	return c
}
