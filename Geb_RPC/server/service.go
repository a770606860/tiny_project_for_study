package server

import (
	"go/ast"
	"log"
	"reflect"
	"sync"
)

type methodType struct {
	method    reflect.Method
	argType   []reflect.Type
	mu        sync.Mutex
	_numCalls uint64
}

func (m *methodType) numCalls() uint64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m._numCalls
}

func (m *methodType) newArgv() []reflect.Value {
	var argvs []reflect.Value
	for _, t := range m.argType {
		var argv reflect.Value
		if t.Kind() == reflect.Ptr {
			argv = reflect.New(t.Elem())
		} else {
			argv = reflect.New(t).Elem()
		}
		argvs = append(argvs, argv)
	}
	return argvs
}

type service struct {
	name    string
	methods map[string]*methodType
	typ     reflect.Type
	rcvr    reflect.Value
}

func (s *service) registerMethods() {
	s.methods = make(map[string]*methodType)
outer:
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mt := method.Type
		if mt.NumOut() > 1 { // 返回值必须为0个或1个
			continue
		}
		if mt.NumOut() == 1 {
			if !isExportedOrBuiltinType(mt) {
				continue
			}
			if mt.PkgPath() != "" && mt.Kind() != reflect.Struct &&
				mt.Kind() != reflect.Slice && mt.Kind() != reflect.Map {
				continue
			}
		}
		var args []reflect.Type
		for i := 0; i < mt.NumIn(); i++ {
			arg := mt.In(i)
			if !isExportedOrBuiltinType(arg) {
				continue outer
			}
			args = append(args, arg)
		}
		s.methods[method.Name] = &methodType{method: method, argType: args}
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func newService(rcvr interface{}) *service {
	var s service
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)
	if !ast.IsExported(s.name) {
		log.Fatalf("rpc server: %s is must to be exported to be a vaild service", s.name)
	}
	s.registerMethods()
	return &s
}

// m must not be nil
func (s *service) call(m *methodType, args ...reflect.Value) interface{} {
	m.mu.Lock()
	m._numCalls++
	m.mu.Unlock()
	f := m.method.Func
	var input []reflect.Value
	input = append(input, s.rcvr)
	input = append(input, args...)
	re := f.Call(input)
	if len(re) == 0 {
		return nil
	}
	return re[0].Interface()
}
