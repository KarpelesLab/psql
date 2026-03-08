package psql

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

// FetchOptions controls the behavior of Fetch, Get, FetchOne, and related operations.
// Use helper constructors [Sort], [Limit], [LimitFrom], [WithPreload], and [FetchLock]
// to create options, or combine multiple options by passing them as variadic arguments.
type FetchOptions struct {
	Lock       bool
	LimitCount int             // number of results to return if >0
	LimitStart int             // seek first record if >0
	Sort       []SortValueable // fields to sort by
	Preload    []string        // association fields to preload after fetching
}

// Sort returns a [FetchOptions] that orders results by the given fields.
// Use [S] to create sort fields: psql.Sort(psql.S("Name", "ASC"))
func Sort(fields ...SortValueable) *FetchOptions {
	return &FetchOptions{Sort: fields}
}

// Limit returns a [FetchOptions] that limits the number of results returned.
func Limit(cnt int) *FetchOptions {
	return &FetchOptions{LimitCount: cnt}
}

// LimitFrom returns a [FetchOptions] with both an offset (start) and a limit (cnt).
// Equivalent to LIMIT cnt OFFSET start in PostgreSQL/SQLite, or LIMIT start, cnt in MySQL.
func LimitFrom(start, cnt int) *FetchOptions {
	return &FetchOptions{
		LimitCount: cnt,
		LimitStart: start,
	}
}

// FetchLock is a [FetchOptions] that adds SELECT ... FOR UPDATE to lock the selected rows.
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
		if len(opt.Preload) > 0 {
			res.Preload = append(res.Preload, opt.Preload...)
		}
	}
	return res
}

// FetchOne loads a single record into target. Unlike [Get], it does not allocate a new
// object; instead it scans into the provided pointer. Returns [os.ErrNotExist] if no
// record matches.
func FetchOne[T any](ctx context.Context, target *T, where any, opts ...*FetchOptions) error {
	return Table[T]().FetchOne(ctx, target, where, opts...)
}

// Get will instanciate a new object of type T and return a pointer to it after loading from database
func Get[T any](ctx context.Context, where any, opts ...*FetchOptions) (*T, error) {
	return Table[T]().Get(ctx, where, opts...)
}

// Fetch returns all records matching the where clause. Pass nil for where to fetch all
// records. Use [FetchOptions] to control sorting, limits, and preloading.
func Fetch[T any](ctx context.Context, where any, opts ...*FetchOptions) ([]*T, error) {
	return Table[T]().Fetch(ctx, where, opts...)
}

// Iter returns a Go 1.23 iterator function that yields records one at a time.
// This is more memory-efficient than [Fetch] for large result sets since rows
// are scanned lazily. Use with range:
//
//	iter, err := psql.Iter[User](ctx, nil)
//	for user := range iter { ... }
func Iter[T any](ctx context.Context, where any, opts ...*FetchOptions) (func(func(v *T) bool), error) {
	return Table[T]().Iter(ctx, where, opts...)
}

func (t *TableMeta[T]) Get(ctx context.Context, where any, opts ...*FetchOptions) (*T, error) {
	if t == nil {
		return nil, ErrNotReady
	}
	t.check(ctx)
	// simplified get
	be := GetBackend(ctx)
	req := B().Select(Raw(t.fldStr)).From(t.FormattedName(be))
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
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:get:run_fail", "psql.table", t.table)
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		// no result
		return nil, os.ErrNotExist
	}
	result, err := t.spawn(ctx, rows)
	// Close rows before preloading to free the connection
	rows.Close()
	if err != nil {
		return nil, err
	}

	if len(opt.Preload) > 0 {
		if err := Preload(ctx, []*T{result}, opt.Preload...); err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (t *TableMeta[T]) FetchOne(ctx context.Context, target *T, where any, opts ...*FetchOptions) error {
	if t == nil {
		return ErrNotReady
	}
	t.check(ctx)
	opt := resolveFetchOpts(opts)

	// grab fields from target
	if target == nil {
		return fmt.Errorf("FetchOne requires a non-nil target")
	}

	// SELECT QUERY
	be := GetBackend(ctx)
	req := B().Select(Raw(t.fldStr)).From(t.FormattedName(be))
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
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:fetch_one:run_fail", "psql.table", t.table)
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		// no result
		return os.ErrNotExist
	}

	err = t.scanValue(ctx, rows, target)
	// Close rows before preloading to free the connection
	rows.Close()
	if err != nil {
		return err
	}

	if len(opt.Preload) > 0 {
		if err := Preload(ctx, []*T{target}, opt.Preload...); err != nil {
			return err
		}
	}

	return nil
}

func (t *TableMeta[T]) Fetch(ctx context.Context, where any, opts ...*FetchOptions) ([]*T, error) {
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
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:fetch:run_fail", "psql.table", t.table)
		return nil, err
	}
	defer rows.Close()

	var final []*T

	for rows.Next() {
		val, err := t.spawn(ctx, rows)
		if err != nil {
			return nil, err
		}
		final = append(final, val)
	}

	if len(opt.Preload) > 0 && len(final) > 0 {
		if err := Preload(ctx, final, opt.Preload...); err != nil {
			return nil, err
		}
	}

	return final, nil
}

func (t *TableMeta[T]) Iter(ctx context.Context, where any, opts ...*FetchOptions) (func(func(v *T) bool), error) {
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
		slog.ErrorContext(ctx, err.Error()+"\n"+debugStack(), "event", "psql:fetch:run_fail", "psql.table", t.table)
		return nil, err
	}

	iterFunc := func(yield func(v *T) bool) {
		defer rows.Close()

		for rows.Next() {
			val, err := t.spawn(ctx, rows)
			if err != nil {
				// iter process has no error reporting method other than panic
				panic(err)
			}
			if !yield(val) {
				return
			}
		}
	}
	return iterFunc, nil
}
