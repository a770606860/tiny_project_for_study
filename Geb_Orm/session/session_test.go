package session

import (
	"database/sql"
	"geborm/clause"
	"geborm/dialect"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"os"
	"sync"
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

func TestSession_Insert(t *testing.T) {
	u1 := &User{Name: "xiaobai", Age: 1}
	u2 := &User{Name: "xiaobao", Age: 1}
	a1 := &Admin{"weiwei", "M"}
	s := NewSession()
	s.DropTable(u1)
	s.CreateTable(u1)
	s.DropTable(a1)
	s.CreateTable(a1)
	n, err := s.Insert(u1, u2, a1)
	assert.Equal(t, int64(3), n)
	assert.Nil(t, err)
}

func TestSession_BatchInsert(t *testing.T) {
	u1 := &User{Name: "qianwan", Age: 3}
	u2 := &User{Name: "xiaobao", Age: 1}
	u3 := &User{Name: "xiaoju", Age: 1}
	s := NewSession()
	s.DropTable(u1)
	s.CreateTable(u1)
	n, err := s.BatchInsert([]interface{}{u1, u2, u3})
	assert.Equal(t, int64(3), n)
	assert.Nil(t, err)
}

func TestSession_Find(t *testing.T) {
	s := insertSomeData(t)
	s.clause.Set(clause.WHERE, "Age > 1")
	u := User{}
	err := s.FindOne(&u)
	assert.Nil(t, err)
	assert.Equal(t, "xiaobao", u.Name)

	var users []User
	err = s.FindAll(&users)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(users))

	var admin Admin
	err = s.FindOne(&admin)
	assert.Nil(t, err)
	assert.Equal(t, "weiwei", admin.Name)

	var admins []Admin
	err = s.FindAll(&admins)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(admins))

}

func insertSomeData(t *testing.T) *Session {
	u1 := &User{Name: "xiaobai", Age: 1}
	u2 := &User{Name: "xiaobao", Age: 2}
	a1 := &Admin{"weiwei", "M"}
	s := NewSession()
	s.DropTable(u1)
	s.CreateTable(u1)
	s.DropTable(a1)
	s.CreateTable(a1)
	n, err := s.Insert(u1, u2, a1)
	assert.Equal(t, int64(3), n)
	assert.Nil(t, err)
	return s
}

func TestSession_Limit(t *testing.T) {
	s := insertSomeData(t)
	var us []User
	err := s.Limit(1).Where("Age >= 1").FindAll(&us)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(us))

	us = []User{}
	err = s.Limit(3).Where("Age >= 1").FindAll(&us)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(us))
}

func TestSession_Count(t *testing.T) {
	s := insertSomeData(t)
	c, err := s.Count(User{})
	assert.Nil(t, err)
	assert.Equal(t, int64(2), c)
	c, err = s.Count(Admin{})
	assert.Nil(t, err)
	assert.Equal(t, int64(1), c)
}

var mu sync.Mutex
var bi, ai, bu, au, bq, aq, bd, ad int

type HookUser struct {
	Name string `geborm:"PRIMARY KEY"`
	Age  int
}

func (u *HookUser) BeforeInsert(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	bi++
}
func (u *HookUser) AfterInsert(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	ai++
}
func (u *HookUser) BeforeUpdate(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	bu++
}
func (u *HookUser) AfterUpdate(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	au++
}
func (u *HookUser) BeforeDelete(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	bd++
}
func (u *HookUser) AfterDelete(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	ad++
}
func (u *HookUser) BeforeQuery(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	bq++
}
func (u *HookUser) AfterQuery(s *Session) {
	mu.Lock()
	defer mu.Unlock()
	aq++
}

func Test_Hook(t *testing.T) {
	// test insert hook
	s := insertSomeData(t)
	h1 := &HookUser{Name: "xiaobai", Age: 1}
	h2 := &HookUser{Name: "xiaobao", Age: 2}
	s.DropTable(h1)
	s.CreateTable(h1)
	_, err := s.Insert(h1, h2)
	assert.Equal(t, 2, bi)
	assert.Equal(t, 2, ai)

	// test update hook
	c, err := s.Where("Name = ?", "xiaobai").Update(&HookUser{}, M{"Age": 2})
	assert.Nil(t, err)
	assert.Equal(t, int64(1), c)
	assert.Equal(t, 1, bu)
	assert.Equal(t, 1, au)

	// test query hook
	u := HookUser{}
	err = s.Where("Name = ?", "xiaobai").FindOne(&u)
	assert.Nil(t, err)
	assert.Equal(t, 1, bq)
	assert.Equal(t, 1, aq)
	assert.Equal(t, 2, u.Age)
	a := Admin{}
	err = s.Where("Name = ?", "wei").FindOne(&a)
	assert.Equal(t, sql.ErrNoRows, err)
	assert.Equal(t, 1, bq)
	assert.Equal(t, 1, aq)
	assert.Equal(t, "", a.Name)

	// test delete hook
	c, err = s.Where("name = ?", "xiaobai").Delete(&HookUser{})
	assert.Nil(t, err)
	assert.Equal(t, int64(1), c)
	c, err = s.Where("name = ?", "weiwei").Delete(&Admin{})
	assert.Nil(t, err)
	assert.Equal(t, int64(1), c)
	assert.Equal(t, 1, bd)
	assert.Equal(t, 1, ad)
}
