package session

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

type User struct {
	Name string `geborm:"PRIMARY KEY"`
	Age  int
}

type Admin struct {
	Name   string `geborm:"PROMARY KEY"`
	Gender string
}

func TestSession_CreateTable(t *testing.T) {
	s := NewSession()
	assert.NotNil(t, s)
	err := s.DropTable(&User{})
	assert.Nil(t, err)
	err = s.CreateTable(&User{})
	assert.Nil(t, err)
	assert.True(t, s.HasTable(&User{}))
}
