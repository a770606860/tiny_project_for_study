package geborm

import (
	"fmt"
	"geborm/session"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"testing"
)

func OpenDB(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine("sqlite3", "geb.db")
	assert.Nil(t, err)
	return e
}

func TestNewEngine(t *testing.T) {
	e := OpenDB(t)
	err := e.Close()
	assert.Nil(t, err)
}

type User struct {
	Name string `geborm:"PRIMARY KEY"`
	Age  int
}

func TestEngine_Transaction(t *testing.T) {
	e := OpenDB(t)
	// test rollback
	s := e.NewSession()
	err := s.DropTable(&User{})
	assert.Nil(t, err)
	err = s.CreateTable(&User{})
	assert.Nil(t, err)
	_, err = e.Transaction(func(s *session.Session) (interface{}, error) {
		u1 := User{Age: 1}
		u2 := User{"xiaoju", 2}
		_, _ = s.Insert(u1, u2)

		return nil, fmt.Errorf("error")
	})
	assert.NotNil(t, err)
	s = e.NewSession()
	var us []User
	err = s.FindAll(&us)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(us))
	// test commit
	_, err = e.Transaction(func(s *session.Session) (interface{}, error) {
		u1 := User{Age: 1}
		u2 := User{"xiaoju", 2}
		_, _ = s.Insert(u1, u2)
		return nil, nil
	})
	assert.Nil(t, err)
	s = e.NewSession()
	err = s.FindAll(&us)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(us))
}
