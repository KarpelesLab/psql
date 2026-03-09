package psql

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
)

// Replace performs an upsert operation: inserts the record if it doesn't exist, or
// replaces it if a conflicting key exists. On MySQL this uses REPLACE INTO, on
// PostgreSQL it uses INSERT ... ON CONFLICT DO UPDATE, on SQLite INSERT OR REPLACE.
// Fires [BeforeSaveHook] and [AfterSaveHook] if implemented.
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
	engine := be.Engine()

	// Get the formatted table name (respects explicit names)
	tableName := t.FormattedName(be)

	// REPLACE QUERY
	ph := engine.Placeholders(len(t.fields), 1)
	var req string

	d := engine.dialect()
	if ur, ok := d.(UpsertRenderer); ok {
		req = ur.ReplaceSQL(tableName, t.fldStr, ph, t.mainKey, t.fields)
	} else {
		// Generic fallback: MySQL-like REPLACE INTO
		if t.mainKey == nil {
			return errors.New("cannot use Replace without a primary key")
		}
		req = "REPLACE INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + ph + ")"
	}

	stmt, err := doPrepareContext(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:replace:prep_fail", "psql.table", tableName)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	for _, target := range targets {
		if h, ok := any(target).(BeforeSaveHook); ok {
			if err := h.BeforeSave(ctx); err != nil {
				return err
			}
		}

		val := reflect.ValueOf(target).Elem()

		params := make([]any, len(t.fields))

		for n, f := range t.fields {
			fval := val.Field(f.Index)
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

		if h, ok := any(target).(AfterSaveHook); ok {
			if err := h.AfterSave(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
