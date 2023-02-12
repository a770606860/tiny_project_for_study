package clause

import "testing"

func TestClause_Select(t *testing.T) {
	var clause Clause
	clause.Set(LIMIT, 3)
	clause.Set(SELECT, "User", []string{"*"})
	clause.Set(ORDERBY, "Age ASC")
	clause.Set(WHERE, "Name = ?", "Tom")
	sql, vars := clause.Build(SELECT, WHERE, ORDERBY, LIMIT)
	t.Log(sql, vars)
}
