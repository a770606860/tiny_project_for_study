package server

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

//被代理的方法必须满足如下条件：
//1，无返回值，或者只有一个返回值且必须为基本类型，结构体，切片或map。这么规定是为了调用方接口清晰
//2，参数和返回类型必须是导出类型或者基本类型
//3，方法本身必须是导出的
// 测试用结构体1
type StudentService struct {
	name string
	age  int
}
type name_ string
type Name string

func (s *StudentService) SetName(name string) {
	s.name = name
}
func (s *StudentService) SetName3(name name_) {
}
func (s *StudentService) SetName4(name Name) {
}
func (s *StudentService) GetName() string {
	return s.name
}
func (s *StudentService) setAge(age int) {
	s.age = age
}
func (s *StudentService) SetName2(name string) (int, error) {
	return 0, nil
}

type NameAndError struct {
	name string
	err  error
}

func (s *StudentService) GetName2() NameAndError {
	return NameAndError{s.name, nil}
}

// 测试用结构体2
type teacher struct {
	name string
}

func Test_Service(t *testing.T) {
	s := newService(&StudentService{})
	assert.Equal(t, 4, len(s.methods))
	re := s.call(s.methods["SetName"], reflect.ValueOf("weiwei"))
	assert.Nil(t, re)
	re = s.call(s.methods["GetName"])
	assert.Equal(t, "weiwei", re.(string))
	re = s.call(s.methods["GetName2"])
	assert.Equal(t, "weiwei", re.(NameAndError).name)

	// s = newService(&teacher{})
}
