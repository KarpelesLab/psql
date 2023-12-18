package psql

import (
	"context"
	"database/sql"
	"log/slog"
)

// Delete will delete values from the table matching the where parameters
func Delete[T any](ctx context.Context, where any, opts ...*FetchOptions) (sql.Result, error) {
	return Table[T]().Delete(ctx, where, opts...)
}

func (t *TableMeta[T]) Delete(ctx context.Context, where any, opts ...*FetchOptions) (sql.Result, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	// simplified get
	req := B().Delete().From(t.table)
	if where != nil {
		req = req.Where(where)
	}

	opt := resolveFetchOpts(opts)
	if opt.LimitCount > 0 {
		if opt.LimitStart > 0 {
			req = req.Limit(opt.LimitStart, opt.LimitCount)
		} else {
			req = req.Limit(opt.LimitCount)
		}
	}

	// run query
	res, err := req.ExecQuery(ctx)
	if err != nil {
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:delete:run_fail", "psql.table", t.table)
		return nil, err
	}
	return res, nil
}
