package session

import (
	"database/sql"
	"geborm/dialect"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

var TestDB, _ = sql.Open("sqlite3", "../geb.db")

func NewSession() *Session {
	d, _ := dialect.GetDialect("sqlite3")
	return New(TestDB, d)
}

func TestMain(m *testing.M) {
	code := m.Run()
	_ = TestDB.Close()
	os.Exit(code)
}

func TestSession_QueryRows(t *testing.T) {
	s := New(TestDB, nil)
	_, err := s.Raw("DROP TABLE IF EXISTS User;").Exec()
	assert.Nil(t, err)
	_, err = s.Raw("CREATE TABLE User(Name text);").Exec()
	assert.Nil(t, err)
	row := s.Raw("SELECT count(*) FROM User").
		QueryRow()
	var count int
	err = row.Scan(&count)
	assert.Nil(t, err)
	assert.Equal(t, 0, count)
	result, err := s.Raw("INSERT INTO User(`name`) values(?), (?)", "Tom", "Sam").Exec()
	c, err := result.RowsAffected()
	assert.Nil(t, err)
	assert.Equal(t, int64(2), c)
}
