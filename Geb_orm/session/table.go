package session

import (
	"geborm/log"
	"geborm/schema"
	"reflect"
)

func (s *Session) Model(value interface{}) *Session {
	if s.refTable == nil ||
		reflect.TypeOf(value) != reflect.TypeOf(s.refTable.Model) {
		s.refTable = schema.Parse(value, s.dialect)
	}
	return s
}

func (s *Session) RefTable() *schema.Schema {
	if s.refTable == nil {
		log.Error("Model is not set")
	}
	return s.refTable
}
