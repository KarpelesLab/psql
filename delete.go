package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Delete will delete values from the table matching the where parameters.
// If the table has a soft delete field (DeletedAt), this performs an UPDATE
// setting the timestamp instead of a hard DELETE. Use [ForceDelete] to bypass
// soft delete.
func Delete[T any](ctx context.Context, where any, opts ...*FetchOptions) (sql.Result, error) {
	return Table[T]().Delete(ctx, where, opts...)
}

func (t *TableMeta[T]) Delete(ctx context.Context, where any, opts ...*FetchOptions) (sql.Result, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	t.check(ctx)
	opt := resolveFetchOpts(opts)

	be := GetBackend(ctx)

	if t.softDelete != nil && !opt.HardDelete {
		// Soft delete: UPDATE SET DeletedAt = NOW()
		req := B().Update(t.FormattedName(be)).
			Set(map[string]any{t.softDelete.Column: time.Now()})
		if where != nil {
			req = req.Where(where)
		}
		// Only soft-delete records that aren't already deleted
		req = req.Where(map[string]any{t.softDelete.Column: nil})

		if opt.LimitCount > 0 {
			if opt.LimitStart > 0 {
				req = req.Limit(opt.LimitStart, opt.LimitCount)
			} else {
				req = req.Limit(opt.LimitCount)
			}
		}

		res, err := req.ExecQuery(ctx)
		if err != nil {
			slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:soft_delete:run_fail", "psql.table", t.table)
			return nil, err
		}
		return res, nil
	}

	// Hard delete
	req := B().Delete().From(t.FormattedName(be))
	if where != nil {
		req = req.Where(where)
	}

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

// DeleteOne will operate the deletion in a separate transaction and ensure only 1 row was deleted or it will
// rollback the deletion and return an error. This is useful when working with important data and security is
// more important than performance.
func DeleteOne[T any](ctx context.Context, where any, opts ...*FetchOptions) error {
	return Table[T]().DeleteOne(ctx, where, opts...)
}

func (t *TableMeta[T]) DeleteOne(ctx context.Context, where any, opts ...*FetchOptions) error {
	t.check(ctx)
	tx, err := BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := t.Delete(ContextTx(ctx, tx), where, opts...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("%w: %d rows where exactly 1 expected", ErrDeleteBadAssert, n)
	}

	return tx.Commit()
}
