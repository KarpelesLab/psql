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
func Insert(ctx context.Context, target ...interface{}) error {
	if len(target) == 0 {
		return nil
	}

	table := GetTableMeta(reflect.TypeOf(target[0]))
	return table.Insert(ctx, target...)
}

func (t *TableMeta) Insert(ctx context.Context, targets ...any) error {
	// INSERT QUERY
	req := "INSERT INTO " + QuoteName(t.table) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	stmt, err := db.PrepareContext(ctx, req)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	for _, target := range targets {
		val := reflect.ValueOf(target)

		for val.Kind() == reflect.Ptr {
			if val.IsNil() {
				// instanciate it
				val.Set(reflect.New(val.Type().Elem()))
			}
			val = val.Elem()
		}
		typ := val.Type()

		if typ != t.typ {
			panic("invalid type for query")
		}

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
