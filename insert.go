package psql

import (
	"context"
	"log"
	"reflect"
	"strings"
)

// Insert is a short way to insert objects into database
//
// psql.Insert(ctx, obj)
//
// Is equivalent to:
//
// psql.Table(obj).Insert(ctx, obj)
//
// All passed objects must be of the same type
func Insert[T any](ctx context.Context, target ...*T) error {
	if len(target) == 0 {
		return nil
	}

	return Table[T]().Insert(ctx, target...)
}

func (t *TableMeta[T]) Insert(ctx context.Context, targets ...*T) error {
	// INSERT QUERY
	req := "INSERT INTO " + QuoteName(t.table) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	stmt, err := doPrepareContext(ctx, req)
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

func InsertIgnore[T any](ctx context.Context, target ...*T) error {
	if len(target) == 0 {
		return nil
	}

	return Table[T]().InsertIgnore(ctx, target...)
}

func (t *TableMeta[T]) InsertIgnore(ctx context.Context, targets ...*T) error {
	// INSERT IGNORE QUERY
	req := "INSERT IGNORE INTO " + QuoteName(t.table) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	stmt, err := doPrepareContext(ctx, req)
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
