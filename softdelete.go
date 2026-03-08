package psql

import (
	"context"
	"database/sql"
	"log/slog"
	"reflect"
	"time"
)

var ptrTimeType = reflect.TypeFor[*time.Time]()

// applySoftDelete adds a WHERE condition to exclude soft-deleted records if the
// table has a soft delete field and WithDeleted is not set.
func (t *TableMeta[T]) applySoftDelete(req *QueryBuilder, opt *FetchOptions) {
	if t.softDelete == nil || (opt != nil && opt.WithDeleted) {
		return
	}
	req.Where(map[string]any{t.softDelete.column: nil})
}

// ForceDelete performs a hard DELETE regardless of whether the table uses soft delete.
func ForceDelete[T any](ctx context.Context, where any, opts ...*FetchOptions) (sql.Result, error) {
	opts = append(opts, &FetchOptions{HardDelete: true})
	return Table[T]().Delete(ctx, where, opts...)
}

// Restore clears the soft delete timestamp on records matching the where clause,
// effectively un-deleting them. Returns [ErrNotReady] if the table has no soft
// delete field.
func Restore[T any](ctx context.Context, where any) (sql.Result, error) {
	return Table[T]().Restore(ctx, where)
}

func (t *TableMeta[T]) Restore(ctx context.Context, where any) (sql.Result, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	if t.softDelete == nil {
		return nil, ErrNotReady
	}
	t.check(ctx)

	be := GetBackend(ctx)
	req := B().Update(t.FormattedName(be)).
		Set(map[string]any{t.softDelete.column: Raw("NULL")}).
		Where(where)
	res, err := req.ExecQuery(ctx)
	if err != nil {
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:restore:run_fail", "psql.table", t.table)
		return nil, err
	}
	return res, nil
}
