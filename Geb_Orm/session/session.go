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

type M map[string]interface{}

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
		s.callHook(BeforeInsert, v)
		result, err := s.Raw(desc, vars...).Exec()
		if err != nil {
			return c, err
		}
		s.callHook(AfterInsert, v)
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
		s.callHook(BeforeInsert, v)
	}
	s.clause.Set(clause.VALUES, vs...)
	desc, vars := s.clause.Build(clause.INSERT, clause.VALUES)
	result, err := s.Raw(desc, vars...).Exec()
	if err != nil {
		return 0, err
	}
	for _, v := range values {
		s.callHook(AfterInsert, v)
	}
	return result.RowsAffected()
}

func (s *Session) FindOne(v interface{}) error {
	r := reflect.Indirect(reflect.ValueOf(v))
	table := s.Model(v).RefTable()
	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	desc, vars := s.clause.Build(clause.SELECT, clause.WHERE,
		clause.ORDERBY, clause.LIMIT)
	s.callHook(BeforeQuery, v)
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
	s.callHook(AfterQuery, v)
	return nil
}

func (s *Session) FindAll(v interface{}) error {
	ds := reflect.Indirect(reflect.ValueOf(v))
	elemT := ds.Type().Elem()
	elem := reflect.New(elemT).Elem().Interface()
	table := s.Model(elem).RefTable()

	s.clause.Set(clause.SELECT, table.Name, table.FieldNames)
	desc, vars := s.clause.Build(clause.SELECT, clause.WHERE, clause.ORDERBY, clause.LIMIT)
	s.callHook(BeforeQuery, elem)
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
		s.callHook(AfterQuery, d.Interface())
	}
	return rows.Close()
}

func (s *Session) Update(table interface{}, kv M) (int64, error) {
	r := s.Model(table).RefTable()
	s.clause.Set(clause.UPDATE, r.Name, (map[string]interface{})(kv))
	desc, vars := s.clause.Build(clause.UPDATE, clause.WHERE)
	s.callHook(BeforeUpdate, table)
	result, err := s.Raw(desc, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.callHook(AfterUpdate, table)
	return result.RowsAffected()
}

func (s *Session) Delete(table interface{}) (int64, error) {
	r := s.Model(table).RefTable()
	s.clause.Set(clause.DELETE, r.Name)
	desc, vars := s.clause.Build(clause.DELETE, clause.WHERE)
	s.callHook(BeforeDelete, table)
	result, err := s.Raw(desc, vars...).Exec()
	if err != nil {
		return 0, err
	}
	s.callHook(AfterDelete, table)
	return result.RowsAffected()
}

func (s *Session) Count(table interface{}) (int64, error) {
	r := s.Model(table).RefTable()
	s.clause.Set(clause.COUNT, r.Name)
	desc, vars := s.clause.Build(clause.COUNT, clause.WHERE)
	s.callHook(BeforeQuery, table)
	row := s.Raw(desc, vars...).QueryRow()
	var tmp int64
	if err := row.Scan(&tmp); err != nil {
		return 0, err
	}
	s.callHook(AfterQuery, table)
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

func (s *Session) callHook(f int, v interface{}) {
	switch f {
	case BeforeUpdate:
		if i, ok := v.(IBeforeUpdate); ok {
			i.BeforeUpdate(s)
		}
	case AfterUpdate:
		if i, ok := v.(IAfterUpdate); ok {
			i.AfterUpdate(s)
		}
	case BeforeDelete:
		if i, ok := v.(IBeforeDelete); ok {
			i.BeforeDelete(s)
		}
	case AfterDelete:
		if i, ok := v.(IAfterDelete); ok {
			i.AfterDelete(s)
		}
	case BeforeInsert:
		if i, ok := v.(IBeforeInsert); ok {
			i.BeforeInsert(s)
		}
	case AfterInsert:
		if i, ok := v.(IAfterInsert); ok {
			i.AfterInsert(s)
		}
	case BeforeQuery:
		if i, ok := v.(IBeforeQuery); ok {
			i.BeforeQuery(s)
		}
	case AfterQuery:
		if i, ok := v.(IAfterQuery); ok {
			i.AfterQuery(s)
		}
	}
}
