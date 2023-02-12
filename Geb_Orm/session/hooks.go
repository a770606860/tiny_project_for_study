package session

type IBeforeQuery interface {
	BeforeQuery(session *Session)
}

type IAfterQuery interface {
	AfterQuery(session *Session)
}

type IBeforeInsert interface {
	BeforeInsert(session *Session)
}

type IAfterInsert interface {
	AfterInsert(session *Session)
}

type IBeforeUpdate interface {
	BeforeUpdate(session *Session)
}

type IAfterUpdate interface {
	AfterUpdate(session *Session)
}

type IBeforeDelete interface {
	BeforeDelete(session *Session)
}

type IAfterDelete interface {
	AfterDelete(session *Session)
}

const (
	BeforeQuery = iota
	AfterQuery
	BeforeUpdate
	AfterUpdate
	BeforeInsert
	AfterInsert
	BeforeDelete
	AfterDelete
)
