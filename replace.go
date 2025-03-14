package psql

import (
	"context"
	"log/slog"
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
	if t == nil {
		return ErrNotReady
	}
	t.check(ctx)

	be := GetBackend(ctx)

	// Format the table name using the namer
	tableName := be.Namer().TableName(t.table)

	// REPLACE QUERY
	req := "REPLACE INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	stmt, err := doPrepareContext(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:replace:prep_fail", "psql.table", tableName)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	engine := be.Engine()

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
			params[n] = engine.export(fval.Interface(), f)
		}

		_, err := stmt.ExecContext(ctx, params...)
		if err != nil {
			slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:replace:run_fail", "psql.table", tableName)
			return &Error{Query: req, Err: err}
		}
	}
	return nil
}
