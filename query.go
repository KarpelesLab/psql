package psql

import (
	"context"
	"database/sql"
	"errors"
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
//
// Deprecated: use .Exec() instead
func Exec(q *SQLQuery) error {
	_, err := GetBackend(nil).DB().Exec(q.Query, q.Args...)
	return err
}

// Query performs a query and use a callback to advance results, meaning there is no need to
// call sql.Rows.Close()
//
// err = psql.Query(psql.Q("SELECT ..."), func(row *sql.Rows) error { ... })
//
// Deprecated: use .Each() instead
func Query(q *SQLQuery, cb func(*sql.Rows) error) error {
	return QueryContext(context.Background(), q, cb)
}

// QueryContext performs a query and use a callback to advance results, meaning there is no need to
// call sql.Rows.Close()
//
// Deprecated: use .Each() instead
func QueryContext(ctx context.Context, q *SQLQuery, cb func(*sql.Rows) error) error {
	r, err := doQueryContext(ctx, q.Query, q.Args...)
	if err != nil {
		return err
	}
	defer r.Close()

	for r.Next() {
		err = cb(r)
		if err != nil {
			if errors.Is(err, ErrBreakLoop) {
				return nil
			}
			return err
		}
	}
	return nil
}

// Each will execute the query and call cb for each row, so you do not need to call
// .Next() or .Close() on the object.
//
// Example use: err := psql.Q("SELECT ...").Each(ctx, func(row *sql.Rows) error { ... })
func (q *SQLQuery) Each(ctx context.Context, cb func(*sql.Rows) error) error {
	r, err := doQueryContext(ctx, q.Query, q.Args...)
	if err != nil {
		return err
	}
	defer r.Close()

	for r.Next() {
		err := cb(r)
		if err != nil {
			if errors.Is(err, ErrBreakLoop) {
				return nil
			}
			return err
		}
	}
	return nil
}

// Exec simply executes the query and returns any error that could have happened
func (q *SQLQuery) Exec(ctx context.Context) error {
	_, err := GetBackend(nil).DB().Exec(q.Query, q.Args...)
	return err
}
