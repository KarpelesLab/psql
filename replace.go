package psql

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strconv"
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
	engine := be.Engine()

	// Get the formatted table name (respects explicit names)
	tableName := t.FormattedName(be)

	// REPLACE QUERY
	var req string

	switch engine {
	case EnginePostgreSQL:
		if t.mainKey == nil {
			return errors.New("cannot use Replace without a primary key on PostgreSQL")
		}
		// INSERT INTO ... ON CONFLICT (key) DO UPDATE SET ...
		req = "INSERT INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES ("
		ln := len(t.fields)
		for i := 0; i < ln; i++ {
			if i > 0 {
				req += ","
			}
			req += "$" + strconv.FormatUint(uint64(i)+1, 10)
		}
		req += ") ON CONFLICT ("
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
	case EngineSQLite:
		req = "INSERT OR REPLACE INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	default: // MySQL
		req = "REPLACE INTO " + QuoteName(tableName) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	}

	stmt, err := doPrepareContext(ctx, req)
	if err != nil {
		slog.ErrorContext(ctx, req+"\n"+err.Error()+"\n"+debugStack(), "event", "psql:replace:prep_fail", "psql.table", tableName)
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
