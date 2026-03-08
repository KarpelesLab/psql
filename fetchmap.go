package psql

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
)

// FetchMapped fetches records and returns them as a map keyed by the string
// representation of the given column. Each key maps to a single record (last wins
// if duplicates exist).
func FetchMapped[T any](ctx context.Context, where any, key string, opts ...*FetchOptions) (map[string]*T, error) {
	return Table[T]().FetchMapped(ctx, where, key, opts...)
}

func (t *TableMeta[T]) FetchMapped(ctx context.Context, where any, key string, opts ...*FetchOptions) (map[string]*T, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	t.check(ctx)
	opt := resolveFetchOpts(opts)

	// SELECT QUERY
	be := GetBackend(ctx)
	req := B().Select(Raw(t.fldStr)).From(t.FormattedName(be))
	if where != nil {
		req = req.Where(where)
	}
	t.applySoftDelete(req, opt)

	if len(opt.Sort) > 0 {
		req = req.OrderBy(opt.Sort...)
	}

	if opt.LimitCount > 0 {
		if opt.LimitStart > 0 {
			req = req.Limit(opt.LimitStart, opt.LimitCount)
		} else {
			req = req.Limit(opt.LimitCount)
		}
	}

	if opt.Lock {
		req.ForUpdate = true
	}
	req = req.Apply(opt.Scopes...)

	// run query
	rows, err := req.RunQuery(ctx)
	if err != nil {
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:fetch_mapped:run_fail", "psql.table", t.table)
		return nil, err
	}
	defer rows.Close()

	final := make(map[string]*T)

	for rows.Next() {
		val, err := t.spawn(ctx, rows)
		if err != nil {
			return nil, err
		}
		st := t.rowstate(val)
		if st == nil {
			return nil, errors.New("object is not appropriate for FetchMapped")
		}
		// TODO avoid using fmt.Sprintf to convert value back to string
		final[fmt.Sprintf("%v", st.val[key])] = val
	}

	return final, nil
}

// FetchGrouped fetches records and returns them grouped by the string
// representation of the given column. Each key maps to a slice of matching records.
func FetchGrouped[T any](ctx context.Context, where map[string]any, key string, opts ...*FetchOptions) (map[string][]*T, error) {
	return Table[T]().FetchGrouped(ctx, where, key, opts...)
}

func (t *TableMeta[T]) FetchGrouped(ctx context.Context, where any, key string, opts ...*FetchOptions) (map[string][]*T, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	t.check(ctx)
	opt := resolveFetchOpts(opts)

	// SELECT QUERY
	be := GetBackend(ctx)
	req := B().Select(Raw(t.fldStr)).From(t.FormattedName(be))
	if where != nil {
		req = req.Where(where)
	}
	t.applySoftDelete(req, opt)

	if len(opt.Sort) > 0 {
		req = req.OrderBy(opt.Sort...)
	}

	if opt.LimitCount > 0 {
		if opt.LimitStart > 0 {
			req = req.Limit(opt.LimitStart, opt.LimitCount)
		} else {
			req = req.Limit(opt.LimitCount)
		}
	}

	if opt.Lock {
		req.ForUpdate = true
	}
	req = req.Apply(opt.Scopes...)

	// run query
	rows, err := req.RunQuery(ctx)
	if err != nil {
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:fetch_grouped:run_fail", "psql.table", t.table)
		return nil, err
	}
	defer rows.Close()

	final := make(map[string][]*T)

	for rows.Next() {
		val, err := t.spawn(ctx, rows)
		if err != nil {
			return nil, err
		}
		st := t.rowstate(val)
		if st == nil {
			return nil, errors.New("object is not appropriate for FetchGrouped")
		}
		// TODO avoid using fmt.Sprintf to convert value back to string
		k := fmt.Sprintf("%v", st.val[key])
		final[k] = append(final[k], val)
	}

	return final, nil
}
