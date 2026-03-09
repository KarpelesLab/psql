package psql

import (
	"context"
	"os"
)

// RunQueryT executes the query and scans all result rows into []*T using the
// table metadata for T. Useful for JOIN queries or custom SELECTs where the
// result maps to a known struct type.
func RunQueryT[T any](ctx context.Context, q *QueryBuilder) ([]*T, error) {
	rows, err := q.RunQuery(ctx)
	if err != nil {
		return nil, err
	}
	return Table[T]().spawnAll(ctx, rows)
}

// RunQueryTOne executes the query and scans a single result row into *T.
// Returns [os.ErrNotExist] if no rows are returned.
func RunQueryTOne[T any](ctx context.Context, q *QueryBuilder) (*T, error) {
	rows, err := q.RunQuery(ctx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, os.ErrNotExist
	}
	return Table[T]().spawn(ctx, rows)
}
