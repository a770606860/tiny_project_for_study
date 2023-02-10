package schema

import (
	"geborm/dialect"
	"github.com/stretchr/testify/assert"
	"testing"
)

type User struct {
	Name string `geborm:"PRIMARY KEY"`
	Age  int
}

func TestParse(t *testing.T) {
	d, ok := dialect.GetDialect("sqlite3")
	assert.True(t, ok)
	s := Parse(&User{}, d)
	assert.Equal(t, "User", s.Name)
	assert.Equal(t, "PRIMARY KEY", s.GetField("Name").Tag)
	assert.Equal(t, 2, len(s.fieldMap))
}
