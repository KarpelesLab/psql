package psql

import (
	"context"
	"fmt"
	"log"
	"os"
)

type FetchOptions struct {
	Lock       bool
	LimitCount int             // number of results to return if >0
	LimitStart int             // seek first record if >0
	Sort       []SortValueable // fields to sort by
}

func Sort(fields ...SortValueable) *FetchOptions {
	return &FetchOptions{Sort: fields}
}

func Limit(cnt int) *FetchOptions {
	return &FetchOptions{LimitCount: cnt}
}

func LimitFrom(start, cnt int) *FetchOptions {
	return &FetchOptions{
		LimitCount: cnt,
		LimitStart: start,
	}
}

var FetchLock = &FetchOptions{Lock: true}

func resolveFetchOpts(opts []*FetchOptions) *FetchOptions {
	res := &FetchOptions{}
	for _, opt := range opts {
		if opt.Lock {
			res.Lock = true
		}
		if opt.LimitCount > 0 {
			res.LimitCount = opt.LimitCount
		}
		if opt.LimitStart > 0 {
			res.LimitStart = opt.LimitStart
		}
		if len(opt.Sort) > 0 {
			res.Sort = append(res.Sort, opt.Sort...)
		}
	}
	return res
}

func FetchOne[T any](ctx context.Context, target *T, where map[string]any, opts ...*FetchOptions) error {
	return Table[T]().FetchOne(ctx, target, where, opts...)
}

// Get will instanciate a new object of type T and return a pointer to it after loading from database
func Get[T any](ctx context.Context, where map[string]any, opts ...*FetchOptions) (*T, error) {
	return Table[T]().Get(ctx, where, opts...)
}

func Fetch[T any](ctx context.Context, where map[string]any, opts ...*FetchOptions) ([]*T, error) {
	return Table[T]().Fetch(ctx, where, opts...)
}

func (t *TableMeta[T]) Get(ctx context.Context, where any, opts ...*FetchOptions) (*T, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	// simplified get
	req := B().Select(Raw(t.fldStr)).From(t.table)
	if where != nil {
		req = req.Where(where)
	}
	req = req.Limit(1)

	opt := resolveFetchOpts(opts)
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

	if !rows.Next() {
		// no result
		return nil, os.ErrNotExist
	}
	return t.spawn(rows)
}

func (t *TableMeta[T]) FetchOne(ctx context.Context, target *T, where any, opts ...*FetchOptions) error {
	if t == nil {
		return ErrNotReady
	}
	opt := resolveFetchOpts(opts)

	// grab fields from target
	if target == nil {
		return fmt.Errorf("FetchOne requires a non-nil target")
	}

	// SELECT QUERY
	req := B().Select(Raw(t.fldStr)).From(t.table)
	if where != nil {
		req = req.Where(where)
	}
	if len(opt.Sort) > 0 {
		req = req.OrderBy(opt.Sort...)
	}

	req = req.Limit(1)
	if opt.Lock {
		req.ForUpdate = true
	}

	// run query
	rows, err := req.RunQuery(ctx)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		// no result
		return os.ErrNotExist
	}

	err = t.scanValue(rows, target)
	return err
}

func (t *TableMeta[T]) Fetch(ctx context.Context, where any, opts ...*FetchOptions) ([]*T, error) {
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

	var final []*T

	for rows.Next() {
		val, err := t.spawn(rows)
		if err != nil {
			return nil, err
		}
		final = append(final, val)
	}

	return final, nil
}
