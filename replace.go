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
	useReturning := false

	switch engine {
	case EnginePostgreSQL:
		if t.mainKey == nil {
			return errors.New("cannot use Replace without a primary key on PostgreSQL")
		}
		// INSERT INTO ... ON CONFLICT (key) DO UPDATE SET ...
		req = "INSERT INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + ph + ") ON CONFLICT ("
		for i, col := range t.mainKey.fields {
			if i > 0 {
				req += ","
			}
			req += QuoteName(col)
		}
		req += ") DO UPDATE SET "
		first := true
		for _, f := range t.fields {
			// skip key fields in SET clause
			isKey := false
			for _, col := range t.mainKey.fields {
				if f.column == col {
					isKey = true
					break
				}
			}
			if isKey {
				continue
			}
			if !first {
				req += ","
			}
			first = false
			req += QuoteName(f.column) + "=EXCLUDED." + QuoteName(f.column)
		}
		req += " RETURNING " + t.fldStr
		useReturning = true
	case EngineSQLite:
		req = "INSERT OR REPLACE INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + ph + ")"
	default: // MySQL
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
			fval := val.Field(f.index)
			if fval.Kind() == reflect.Ptr {
				if fval.IsNil() {
					continue
				}
			}
			params[n] = engine.export(fval.Interface(), f)
		}

		if useReturning {
			rows, err := stmt.QueryContext(ctx, params...)
			if err != nil {
				slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:replace:run_fail", "psql.table", tableName)
				return &Error{Query: req, Err: err}
			}
			if rows.Next() {
				if err := t.scanValueReturning(ctx, rows, target); err != nil {
					rows.Close()
					return err
				}
			}
			rows.Close()
		} else {
			_, err := stmt.ExecContext(ctx, params...)
			if err != nil {
				slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:replace:run_fail", "psql.table", tableName)
				return &Error{Query: req, Err: err}
			}
		}

		if h, ok := any(target).(AfterSaveHook); ok {
			if err := h.AfterSave(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
