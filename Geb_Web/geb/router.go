package geb

import (
	"fmt"
	"strings"
)

type Router interface {
	AddHandler(method, patten string, handler HandlerFunc) error
	Handlers(method, path string) []HandlerFunc
	AddMiddlewire(path string, handler ...HandlerFunc) error
}

// 支持3种模式：
//
// 1，精确匹配，/a/b/c
//
// 2，前缀匹配，/a/b/*，"*"代表一个或多个路径
//
// /a/b或者/a/b/c的匹配结果为真
//
// 3，"."匹配，/a/./b，"."不可以是最后一个字符
//
// /a/d/b匹配结果为真，/a/c和/a/c/d/b匹配结果为假
//
// 当多个模式均匹配时，返回匹配度最高的结果，/a/*和/a/b/*，/a/b/c均可匹配/a/b/c，
// 此时匹配结果为/a/b/c
type defaultRouter struct {
	root *node
}
type node struct {
	part     string
	midwirs  []HandlerFunc
	handlers []HandlerFunc
	children []*node
}

const (
	GET  = 0
	POST = 1
)

func methodToInt(method string) int {
	m := -1
	switch method {
	case "GET":
		m = GET
	case "POST":
		m = POST
	}
	return m
}
func isValidPatten(patten string) bool {
	if len(patten) == 0 || patten[0] != '/' {
		return false
	}
	if len(patten) > 1 && patten[len(patten)-1] == '/' {
		patten = patten[:len(patten)-1]
	}
	star := strings.Index(patten, "*")
	dot := strings.Index(patten, ".")
	l := len(patten)
	if (star != -1 && star != l-1) || dot == l-1 || (star >= 0 && dot >= 0) {
		return false
	}
	if dot != -1 {
		dot = strings.Index(patten[dot+1:], ".")
		if dot != -1 {
			return false
		}
	}
	return true
}
func splitP(p string) []string {
	ps := strings.Split(p, "/")
	l := len(ps)
	if l < 2 {
		return nil
	}
	if ps[l-1] == "" {
		l--
	}
	ps[0] = "/"
	return ps[:l]
}
func (router *defaultRouter) AddHandler(method, patten string, handler HandlerFunc) error {
	m := methodToInt(method)
	if m < 0 {
		return fmt.Errorf("无法识别的方法%s", method)
	}
	if !isValidPatten(patten) {
		return fmt.Errorf("无法识别的模式%s", patten)
	}
	ps := splitP(patten)
	if len(ps) == 0 {
		return fmt.Errorf("无法识别的模式%s", patten)
	}
	n := updateTree(router, ps)
	if m > len(n.handlers)-1 {
		ha := make([]HandlerFunc, m+1)
		for i, ff := range n.handlers {
			ha[i] = ff
		}
		n.handlers = ha
	}
	n.handlers[m] = handler
	return nil
}
func updateTree(router *defaultRouter, ps []string) *node {
	if router.root == nil {
		router.root = &node{part: "/"}
	}
	n := router.root
outer:
	for _, p := range ps[1:] {
		for _, nn := range n.children {
			if nn.part == p {
				n = nn
				continue outer
			}
		}
		n3 := node{part: p}
		n.children = append(n.children, &n3)
		n = &n3
	}
	return n
}
func (router *defaultRouter) Handlers(method, path string) (handlers []HandlerFunc) {
	if router.root == nil {
		return nil
	}
	m := methodToInt(method)
	if m < 0 {
		return nil
	}
	ps := splitP(path)
	if len(ps) == 0 {
		return nil
	}
	handlers, has := doGetHandlers(router.root, ps, m)
	if !has {
		return nil
	}
	return handlers
}
func doGetHandlers(n *node, ps []string, m int) ([]HandlerFunc, bool) {
	if n == nil || len(ps) == 0 {
		return nil, false
	}
	if n.part == "." {
		for _, nn := range n.children {
			p, hasHandler := doGetHandlers(nn, ps[1:], m)
			if hasHandler {
				return p, true
			}
		}
		return nil, false
	}
	if n.part == "*" {
		if m < len(n.handlers) && n.handlers[m] != nil {
			handlers := []HandlerFunc{n.handlers[m]}
			return handlers, true
		}
		return nil, false
	}
	if n.part == ps[0] {
		var handlers []HandlerFunc
		handlers = append(handlers, n.midwirs...)
		if len(ps) == 1 {
			if m >= len(n.handlers) || n.handlers[m] == nil {
				return handlers, false
			}
			return append(handlers, n.handlers[m]), true
		}

		var h1, h2, h3 []HandlerFunc
		var has1, has2, has3 bool
		for _, nn := range n.children {
			if nn.part == "*" {
				h1, has1 = doGetHandlers(nn, ps[1:], m)
			} else if nn.part == "." {
				h2, has2 = doGetHandlers(nn, ps[1:], m)
			} else if nn.part == ps[1] {
				h3, has3 = doGetHandlers(nn, ps[1:], m)
			}
		}

		if has3 {
			return append(handlers, h3...), true
		} else if has2 {
			return append(handlers, h2...), true
		} else if has1 {
			handlers = append(handlers, h3...)
			handlers = append(handlers, h1...)
			return handlers, true
		} else {
			return h3, false
		}
	}
	return nil, false
}
func (router *defaultRouter) AddMiddlewire(path string, handler ...HandlerFunc) error {
	ps := splitP(path)
	if len(path) > 0 && path[0] != '/' {
		return fmt.Errorf("路径必须以/开头")
	}
	if len(ps) == 0 || strings.Index(path, ".") >= 0 || strings.Index(path, "*") >= 0 {
		return fmt.Errorf("路径格式错误%s", path)
	}
	n := updateTree(router, ps)
	n.midwirs = append(n.midwirs, handler...)
	return nil
}
