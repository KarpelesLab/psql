package psql

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

func FetchOne[T any](ctx context.Context, target *T, where map[string]any) error {
	return Table[T]().FetchOne(ctx, target, where)
}

// Get will instanciate a new object of type T and return a pointer to it after loading from database
func Get[T any](ctx context.Context, where map[string]any) (*T, error) {
	return Table[T]().Get(ctx, where)
}

func Fetch[T any](ctx context.Context, where map[string]any) ([]*T, error) {
	return Table[T]().Fetch(ctx, where)
}

func (t *TableMeta[T]) Get(ctx context.Context, where map[string]interface{}) (*T, error) {
	// simplified get
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []interface{}
	if where != nil {
		var whQ []string
		for k, v := range where {
			whQ = append(whQ, QuoteName(k)+"=?")
			params = append(params, v)
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}
	req += " LIMIT 1"

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

func (t *TableMeta[T]) FetchOne(ctx context.Context, target *T, where map[string]interface{}) error {
	// grab fields from target
	if target == nil {
		return fmt.Errorf("FetchOne requires a non-nil target")
	}

	// SELECT QUERY
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []interface{}
	if where != nil {
		var whQ []string
		for k, v := range where {
			whQ = append(whQ, QuoteName(k)+"=?")
			params = append(params, v)
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}
	req += " LIMIT 1"

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

func (t *TableMeta[T]) Fetch(ctx context.Context, where map[string]interface{}) ([]*T, error) {
	// SELECT QUERY
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []interface{}
	if where != nil {
		var whQ []string
		for k, v := range where {
			whQ = append(whQ, QuoteName(k)+"=?")
			params = append(params, v)
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
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
