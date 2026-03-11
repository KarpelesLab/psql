package psql

import (
	"context"
	"log/slog"
	"reflect"
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
	if t == nil {
		return ErrNotReady
	}
	t.check(ctx)

	be := GetBackend(ctx)
	engine := be.Engine()

	// Get the formatted table name (respects explicit names)
	tableName := t.FormattedName(be)

	// INSERT QUERY
	req := "INSERT INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + engine.Placeholders(len(t.fields), 1) + ")"

	d := engine.dialect()
	useReturning := false
	if rr, ok := d.(ReturningRenderer); ok {
		useReturning = rr.SupportsReturning()
	}
	if useReturning {
		req += " RETURNING " + t.fldStr
	}

	stmt, err := doPrepareContext(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:insert:prep_fail", "psql.table", tableName)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	for _, target := range targets {
		if h, ok := any(target).(BeforeSaveHook); ok {
			if err := h.BeforeSave(ctx); err != nil {
				return err
			}
		}
		if h, ok := any(target).(BeforeInsertHook); ok {
			if err := h.BeforeInsert(ctx); err != nil {
				return err
			}
		}

		val := reflect.ValueOf(target).Elem()

		params := make([]any, len(t.fields))

		for n, f := range t.fields {
			fval := val.Field(f.Index)
			switch fval.Kind() {
			case reflect.Ptr, reflect.Slice, reflect.Map:
				if fval.IsNil() {
					continue
				}
			}
			params[n] = engine.export(fval.Interface(), f)
		}

		if useReturning {
			rows, err := stmt.QueryContext(ctx, params...)
			if err != nil {
				slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:insert:run_fail", "psql.table", tableName)
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
				slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:insert:run_fail", "psql.table", tableName)
				return &Error{Query: req, Err: err}
			}
		}

		if h, ok := any(target).(AfterInsertHook); ok {
			if err := h.AfterInsert(ctx); err != nil {
				return err
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

// InsertIgnore inserts records, silently ignoring conflicts (e.g., duplicate keys).
// On PostgreSQL this uses ON CONFLICT DO NOTHING, on MySQL INSERT IGNORE, on SQLite
// INSERT OR IGNORE. Hooks are called the same as [Insert].
func InsertIgnore[T any](ctx context.Context, target ...*T) error {
	if len(target) == 0 {
		return nil
	}

	return Table[T]().InsertIgnore(ctx, target...)
}

func (t *TableMeta[T]) InsertIgnore(ctx context.Context, targets ...*T) error {
	if t == nil {
		return ErrNotReady
	}
	t.check(ctx)

	be := GetBackend(ctx)
	engine := be.Engine()

	// Get the formatted table name (respects explicit names)
	tableName := t.FormattedName(be)

	// INSERT IGNORE QUERY
	ph := engine.Placeholders(len(t.fields), 1)
	var req string

	d := engine.dialect()
	useReturning := false
	if rr, ok := d.(ReturningRenderer); ok {
		useReturning = rr.SupportsReturning()
	}

	if ur, ok := d.(UpsertRenderer); ok {
		req = ur.InsertIgnoreSQL(tableName, t.fldStr, ph)
	} else {
		// Generic fallback: MySQL-like INSERT IGNORE
		req = "INSERT IGNORE INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + ph + ")"
	}

	if useReturning {
		req += " RETURNING " + t.fldStr
	}

	stmt, err := doPrepareContext(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:insert_ignore:prep_fail", "psql.table", tableName)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	for _, target := range targets {
		if h, ok := any(target).(BeforeSaveHook); ok {
			if err := h.BeforeSave(ctx); err != nil {
				return err
			}
		}
		if h, ok := any(target).(BeforeInsertHook); ok {
			if err := h.BeforeInsert(ctx); err != nil {
				return err
			}
		}

		val := reflect.ValueOf(target).Elem()

		params := make([]any, len(t.fields))

		for n, f := range t.fields {
			fval := val.Field(f.Index)
			switch fval.Kind() {
			case reflect.Ptr, reflect.Slice, reflect.Map:
				if fval.IsNil() {
					continue
				}
			}
			params[n] = engine.export(fval.Interface(), f)
		}

		if useReturning {
			rows, err := stmt.QueryContext(ctx, params...)
			if err != nil {
				slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:insert_ignore:run_fail", "psql.table", tableName)
				return &Error{Query: req, Err: err}
			}
			// ON CONFLICT DO NOTHING may produce no rows if conflict occurred
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
				slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:insert_ignore:run_fail", "psql.table", tableName)
				return &Error{Query: req, Err: err}
			}
		}

		if h, ok := any(target).(AfterInsertHook); ok {
			if err := h.AfterInsert(ctx); err != nil {
				return err
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
