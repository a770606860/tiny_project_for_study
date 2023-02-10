package session

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type User struct {
	Name string `geborm:"PRIMARY KEY"`
	Age  int
}

func TestSession_CreateTable(t *testing.T) {
	s := NewSession()
	assert.NotNil(t, s)
	s.Model(&User{})
	err := s.DropTable()
	assert.Nil(t, err)
	err = s.CreateTable()
	assert.Nil(t, err)
	assert.True(t, s.HasTable())

}
