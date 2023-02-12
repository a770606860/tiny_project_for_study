package session

import (
	"database/sql"
	"fmt"
	"geborm/clause"
	"geborm/dialect"
	"geborm/log"
	"geborm/schema"
	"reflect"
	"strings"
)

type Session struct {
	db       *sql.DB
	dialect  dialect.Dialect
	refTable *schema.Schema // 缓存表结构
	clause   clause.Clause
	sql      strings.Builder
	sqlVars  []interface{}
}

func New(db *sql.DB, d dialect.Dialect) *Session {
	return &Session{db: db,
		dialect: d}
}

func (s *Session) Clear() {
	s.sql.Reset()
	s.clause.Reset()
	s.sqlVars = nil

}

func (s *Session) DB() *sql.DB {
	return s.db
}

func (s *Session) Raw(sql string, values ...interface{}) *Session {
	s.sql.WriteString(sql)
	s.sql.WriteString(" ")
	s.sqlVars = append(s.sqlVars, values...)
	return s
}

func (s *Session) Exec() (result sql.Result, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if result, err = s.DB().Exec(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return result, err
}

func (s *Session) QueryRow() *sql.Row {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	return s.DB().QueryRow(s.sql.String(), s.sqlVars...)
}

func (s *Session) QueryRows() (rows *sql.Rows, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if rows, err = s.DB().Query(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return
}

func (s *Session) CreateTable(table interface{}) error {
	t := s.Model(table).RefTable()
	var columns []string
	for _, field := range t.Fields {
		columns = append(columns,
			fmt.Sprintf("%s %s %s", field.Name, field.Type, field.Tag))
	}
	desc := strings.Join(columns, ",")
	_, err := s.Raw(fmt.Sprintf("CREATE TABLE %s (%s);", t.Name, desc)).Exec()
	return err
}

func (s *Session) DropTable(table interface{}) error {
	s.Model(table)
	_, err := s.Raw(fmt.Sprintf("DROP TABLE IF EXISTS %s;", s.RefTable().Name)).Exec()
	return err
}

func (s *Session) HasTable(table interface{}) bool {
	desc, values := s.dialect.TableExistSQL(s.Model(table).RefTable().Name)
	row := s.Raw(desc, values...).QueryRow()
	var temp string
	err := row.Scan(&temp)
	if err != nil {
		log.Error(err)
	}
	return temp == s.RefTable().Name
}

func (s *Session) Insert(values ...interface{}) (int64, error) {
	var c int64
	for _, v := range values {
		r := s.Model(v).RefTable()
		s.clause.Set(clause.INSERT, r.Name, r.FieldNames)
		s.clause.Set(clause.VALUES, s.RefTable().RecordValues(v))
		desc, vars := s.clause.Build(clause.INSERT, clause.VALUES)
		result, err := s.Raw(desc, vars...).Exec()
		if err != nil {
			return c, err
		}
		cc, _ := result.RowsAffected()
		c += cc
	}
	return c, nil
}

func (s *Session) BatchInsert(values []interface{}) (int64, error) {
	if len(values) == 0 {
		return 0, nil
	}
	r := s.Model(values[0]).RefTable()
	s.clause.Set(clause.INSERT, r.Name, r.FieldNames)
	var vs []interface{}
	for _, v := range values {
		vs = append(vs, s.RefTable().RecordValues(v))
	}
	s.clause.Set(clause.VALUES, vs...)
	desc, vars := s.clause.Build(clause.INSERT, clause.VALUES)
	result, err := s.Raw(desc, vars...).Exec()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Session) FindOne(v interface{}) error {
	r := reflect.Indirect(reflect.ValueOf(v))
	table := s.Model(v).RefTable()
	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	desc, vars := s.clause.Build(clause.SELECT, clause.WHERE,
		clause.ORDERBY, clause.LIMIT)
	row := s.Raw(desc, vars...).QueryRow()
	if err := row.Err(); err != nil {
		return err
	}
	var vs []interface{}
	for _, name := range table.FieldNames {
		vs = append(vs, r.FieldByName(name).Addr().Interface())
	}
	if err := row.Scan(vs...); err != nil {
		return err
	}
	return nil
}

func (s *Session) FindAll(v interface{}) error {
	ds := reflect.Indirect(reflect.ValueOf(v))
	elemT := ds.Type().Elem()
	table := s.Model(reflect.New(elemT).Elem().Interface()).RefTable()

	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	desc, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	rows, err := s.Raw(desc, vars...).QueryRows()
	if err != nil {
		return err
	}

	for rows.Next() {
		d := reflect.New(elemT).Elem()
		var vs []interface{}
		for _, name := range table.FieldNames {
			vs = append(vs, d.FieldByName(name).Addr().Interface())
		}
		if err = rows.Scan(vs...); err != nil {
			return err
		}
		ds.Set(reflect.Append(ds, d))
	}
	return rows.Close()
}

func (s *Session) Update(table interface{}, kv map[string]interface{}) (int64, error) {
	r := s.Model(table).RefTable()
	s.clause.Set(clause.UPDATE, r.Name, kv)
	desc, vars := s.clause.Build(clause.UPDATE, clause.WHERE)
	result, err := s.Raw(desc, vars).Exec()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Session) Delete(table interface{}) (int64, error) {
	r := s.Model(table).RefTable()
	s.clause.Set(clause.DELETE, r.Name)
	desc, vars := s.clause.Build(clause.DELETE, clause.WHERE)
	result, err := s.Raw(desc, vars...).Exec()
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *Session) Count(table interface{}) (int64, error) {
	r := s.Model(table).RefTable()
	s.clause.Set(clause.COUNT, r.Name)
	desc, vars := s.clause.Build(clause.COUNT, clause.WHERE)
	row := s.Raw(desc, vars...).QueryRow()
	var tmp int64
	if err := row.Scan(&tmp); err != nil {
		return 0, err
	}
	return tmp, nil
}

func (s *Session) Limit(num int) *Session {
	s.clause.Set(clause.LIMIT, num)
	return s
}

func (s *Session) Where(desc string, args ...interface{}) *Session {
	var vars []interface{}
	vars = append(vars, desc)
	vars = append(vars, args...)
	s.clause.Set(clause.WHERE, vars...)
	return s
}

func (s *Session) OrderBy(desc string) *Session {
	s.clause.Set(clause.ORDERBY, desc)
	return s
}
