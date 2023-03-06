package psql

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
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

func (t *TableMeta[T]) Get(ctx context.Context, where map[string]any, opts ...*FetchOptions) (*T, error) {
	// simplified get
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []any
	if where != nil {
		var whQ []string
		for k, v := range where {
			switch rv := v.(type) {
			case []string:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			case []any:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			default:
				whQ = append(whQ, QuoteName(k)+"=?")
				params = append(params, v)
			}
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}
	req += " LIMIT 1"

	opt := resolveFetchOpts(opts)
	if opt.Lock {
		req += " FOR UPDATE"
	}

	// run query
	rows, err := doQueryContext(ctx, req, params...)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return nil, &Error{Query: req, Err: err}
	}
	defer rows.Close()

	if !rows.Next() {
		// no result
		return nil, os.ErrNotExist
	}
	return t.spawn(rows)
}

func (t *TableMeta[T]) FetchOne(ctx context.Context, target *T, where map[string]any, opts ...*FetchOptions) error {
	opt := resolveFetchOpts(opts)

	// grab fields from target
	if target == nil {
		return fmt.Errorf("FetchOne requires a non-nil target")
	}

	// SELECT QUERY
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []any
	if where != nil {
		var whQ []string
		for k, v := range where {
			switch rv := v.(type) {
			case []string:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			case []any:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			default:
				whQ = append(whQ, QuoteName(k)+"=?")
				params = append(params, v)
			}
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}
	req += " LIMIT 1"
	if opt.Lock {
		req += " FOR UPDATE"
	}

	// run query
	rows, err := doQueryContext(ctx, req, params...)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return &Error{Query: req, Err: err}
	}
	defer rows.Close()

	if !rows.Next() {
		// no result
		return os.ErrNotExist
	}

	err = t.scanValue(rows, target)
	return err
}

func (t *TableMeta[T]) Fetch(ctx context.Context, where map[string]any, opts ...*FetchOptions) ([]*T, error) {
	opt := resolveFetchOpts(opts)

	// SELECT QUERY
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []any
	if where != nil {
		var whQ []string
		for k, v := range where {
			switch rv := v.(type) {
			case []string:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			case []any:
				// IN (...)
				repQ := make([]string, len(rv))
				for n := range repQ {
					repQ[n] = "?"
				}
				whQ = append(whQ, QuoteName(k)+" IN ("+strings.Join(repQ, ",")+")")
				for _, sv := range rv {
					params = append(params, sv)
				}
			default:
				whQ = append(whQ, QuoteName(k)+"=?")
				params = append(params, v)
			}
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}

	if len(opt.Sort) > 0 {
		// add sort (TODO)
		req += "ORDER BY "
		for n, o := range opt.Sort {
			if n > 0 {
				req += ", "
			}
			req += o.sortEscapeValue()
		}
	}

	if opt.LimitCount > 0 {
		if opt.LimitStart > 0 {
			req += fmt.Sprintf(" LIMIT %d, %d", opt.LimitStart, opt.LimitCount)
		} else {
			req += fmt.Sprintf(" LIMIT %d", opt.LimitCount)
		}
	}

	if opt.Lock {
		req += " FOR UPDATE"
	}

	// run query
	rows, err := doQueryContext(ctx, req, params...)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return nil, &Error{Query: req, Err: err}
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
