package mysql

import (
	"context"
	"database/sql"
)

// DBExecuter wraps transaction and non-transaction db calls
type DBExecuter interface {
	Invalidate(func()) error
	Query(ctx context.Context, unprepared string, args ...interface{}) (*sql.Rows, error)
	Exec(ctx context.Context, unprepared string, args ...interface{}) (sql.Result, error)
	Prepare(ctx context.Context, query string) (*sql.Stmt, error)
}

type RowScanner interface {
	Scan(dest ...interface{}) error
}

func NonEmptyOrNil(str string) interface{} {
	if str == "" {
		return nil
	}
	return str
}
