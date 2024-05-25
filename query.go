package psql

import (
	"context"
	"database/sql"
)

type SQLQuery struct {
	Query string
	Args  []any
}

// Q is a short hand to create a Query object
func Q(q string, args ...any) *SQLQuery {
	return &SQLQuery{q, args}
}

// Exec simply runs a query against the DefaultBackend
func Exec(q *SQLQuery) error {
	_, err := GetBackend(nil).DB().Exec(q.Query, q.Args...)
	return err
}

// Query performs a query and use a callback to advance results, meaning there is no need to
// call sql.Rows.Close()
//
// err = psql.Query(psql.Q("SELECT ..."), func(row *sql.Rows) error { ... })
func Query(q *SQLQuery, cb func(*sql.Rows) error) error {
	return QueryContext(context.Background(), q, cb)
}

func QueryContext(ctx context.Context, q *SQLQuery, cb func(*sql.Rows) error) error {
	r, err := doQueryContext(ctx, q.Query, q.Args...)
	if err != nil {
		return err
	}
	defer r.Close()

	for r.Next() {
		err = cb(r)
		if err != nil {
			return err
		}
	}
	return nil
}
