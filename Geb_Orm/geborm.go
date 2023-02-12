package geborm

import (
	"database/sql"
	"geborm/dialect"
	"geborm/log"
	"geborm/session"
)

type Engine struct {
	db      *sql.DB
	dialect dialect.Dialect
}

func NewEngine(driver, source string) (e *Engine, err error) {
	db, err := sql.Open(driver, source)
	if err != nil {
		log.Error(err)
		return
	}
	if err = db.Ping(); err != nil {
		log.Error(err)
		return
	}
	d, ok := dialect.GetDialect(driver)
	if !ok {
		log.Errorf("dialect %s Not Found", driver)
		return
	}
	e = &Engine{db: db, dialect: d}
	log.Info("Connect database success")
	return
}

func (e *Engine) NewSession() *session.Session {
	return session.New(e.db, e.dialect)
}

func (e *Engine) Close() error {
	return e.db.Close()
}

type TxFunc func(session2 *session.Session) (interface{}, error)

func (e *Engine) Transaction(f TxFunc) (result interface{}, err error) {
	s := e.NewSession()
	if err := s.Begin(); err != nil {
		return nil, err
	}
	defer func() {
		if p := recover(); p != nil {
			err2 := s.Rollback()
			if err2 != nil {
				log.Error(err2)
			}
			panic(p)
		} else if err != nil {
			err2 := s.Rollback()
			if err2 != nil {
				log.Error(err2)
			}
		} else {
			err = s.Commit()
			if err != nil {
				err2 := s.Rollback()
				if err2 != nil {
					log.Error(err2)
				}
			}
		}
	}()
	return f(s)
}
