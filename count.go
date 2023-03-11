package psql

import (
	"context"
	"os"
)

func Count[T any](ctx context.Context, where map[string]any) (int, error) {
	return Table[T]().Count(ctx, where)
}

func (t *TableMeta[T]) Count(ctx context.Context, where map[string]any) (int, error) {
	// simplified get
	req := B().Select(Raw("COUNT(1)")).From(t.table)
	if where != nil {
		req = req.Where(where)
	}

	// run query
	rows, err := req.RunQuery(ctx)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	if !rows.Next() {
		// should not happen with COUNT(1)
		return 0, os.ErrNotExist
	}

	var res int
	err = rows.Scan(&res)

	return res, err
}
