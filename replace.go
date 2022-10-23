package psql

import (
	"context"
	"log"
	"reflect"
	"strings"
)

// Replace is a short way to replace objects into database
//
// psql.Replace(ctx, obj)
//
// Is equivalent to:
//
// psql.Table(obj).Replace(ctx, obj)
//
// All passed objects must be of the same type
func Replace[T any](ctx context.Context, target ...*T) error {
	if len(target) == 0 {
		return nil
	}

	return Table[T]().Replace(ctx, target...)
}

func (t *TableMeta[T]) Replace(ctx context.Context, targets ...*T) error {
	// REPLACE QUERY
	req := "REPLACE INTO " + QuoteName(t.table) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	stmt, err := db.PrepareContext(ctx, req)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	for _, target := range targets {
		val := reflect.ValueOf(target).Elem()

		params := make([]any, len(t.fields))

		for n, f := range t.fields {
			fval := val.Field(f.index)
			if fval.Kind() == reflect.Ptr {
				if fval.IsNil() {
					continue
				}
			}
			params[n] = export(fval.Interface())
		}

		_, err := stmt.ExecContext(ctx, params...)
		if err != nil {
			log.Printf("[sql] error: %s", err)
			return &Error{Query: req, Err: err}
		}
	}
	return nil
}
