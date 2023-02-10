package geb

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"sync"
)

type Engine struct {
	htmlTemplate *template.Template
	router       Router

	started bool
	lock    sync.Mutex
}

func New() *Engine {
	// templ := template.Must(template.New("").ParseGlob("templates/*"))
	return &Engine{htmlTemplate: nil, router: &defaultRouter{}}
}

func hasRun(e *Engine) error {
	e.lock.Lock()
	if e.started {
		e.lock.Unlock()
		return fmt.Errorf("engine has started")
	}
	e.lock.Unlock()
	return nil
}

func (e *Engine) Template(templ *template.Template) error {
	if err := hasRun(e); err != nil {
		return err
	}
	e.htmlTemplate = templ
	return nil
}

func (e *Engine) Run(addr string) (err error) {
	e.lock.Lock()
	if e.started {
		e.lock.Unlock()
		return fmt.Errorf("engine has started")
	} else {
		e.started = true
		e.lock.Unlock()
	}
	return http.ListenAndServe(addr, e)
}

func (e *Engine) GET(patten string, handler HandlerFunc) error {
	return e.router.AddHandler("GET", patten, handler)
}
func (e *Engine) POST(patten string, handler HandlerFunc) error {
	return e.router.AddHandler("POST", patten, handler)
}

func (e *Engine) AddMiddlewire(path string, handler ...HandlerFunc) error {
	return e.router.AddMiddlewire(path, handler...)
}

func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	handler := e.router.Handlers(req.Method, req.URL.Path)
	ctx := newCtx(w, req, handler)
	if len(handler) == 0 {
		log.Printf("路径错误：%s", req.Method+"-"+req.URL.Path)
		ctx.Fail(500)
		if _, err := ctx.Data(http.StatusNotFound, nil); err != nil {
			log.Printf(err.Error())
		}
	} else {
		ctx.run()
	}
}
