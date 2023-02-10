package geborm

import (
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
