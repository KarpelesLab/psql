package psql

import (
	"context"
	"errors"
	"fmt"
	"log"
)

func FetchMapped[T any](ctx context.Context, where map[string]any, key string, opts ...*FetchOptions) (map[string]*T, error) {
	return Table[T]().FetchMapped(ctx, where, key, opts...)
}

func (t *TableMeta[T]) FetchMapped(ctx context.Context, where any, key string, opts ...*FetchOptions) (map[string]*T, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	opt := resolveFetchOpts(opts)

	// SELECT QUERY
	req := B().Select(Raw(t.fldStr)).From(t.table)
	if where != nil {
		req = req.Where(where)
	}

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

	// run query
	rows, err := req.RunQuery(ctx)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return nil, err
	}
	defer rows.Close()

	final := make(map[string]*T)

	for rows.Next() {
		val, err := t.spawn(rows)
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

func FetchGrouped[T any](ctx context.Context, where map[string]any, key string, opts ...*FetchOptions) (map[string][]*T, error) {
	return Table[T]().FetchGrouped(ctx, where, key, opts...)
}

func (t *TableMeta[T]) FetchGrouped(ctx context.Context, where any, key string, opts ...*FetchOptions) (map[string][]*T, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	opt := resolveFetchOpts(opts)

	// SELECT QUERY
	req := B().Select(Raw(t.fldStr)).From(t.table)
	if where != nil {
		req = req.Where(where)
	}

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

	// run query
	rows, err := req.RunQuery(ctx)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return nil, err
	}
	defer rows.Close()

	final := make(map[string][]*T)

	for rows.Next() {
		val, err := t.spawn(rows)
		if err != nil {
			return nil, err
		}
		st := t.rowstate(val)
		if st == nil {
			return nil, errors.New("object is not appropriate for FetchMapped")
		}
		// TODO avoid using fmt.Sprintf to convert value back to string
		k := fmt.Sprintf("%v", st.val[key])
		final[k] = append(final[k], val)
	}

	return final, nil
}
