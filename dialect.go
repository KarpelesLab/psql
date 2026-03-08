package psql

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Dialect defines engine-specific SQL behaviors. Each database engine registers
// its own Dialect implementation via [RegisterDialect]. This interface is the
// foundation for moving backend-specific logic into separate submodules.
type Dialect interface {
	// Placeholder returns the SQL placeholder for the nth parameter (1-indexed).
	// For example, MySQL/SQLite return "?" (ignoring n), PostgreSQL returns "$1", "$2", etc.
	Placeholder(n int) string

	// ExportArg transforms a Go value for use as a query parameter.
	// This handles engine-specific formatting (e.g., time.Time representation).
	ExportArg(v any) any

	// LimitOffset renders a two-argument LIMIT clause. The arguments are passed
	// through as stored by [QueryBuilder.Limit](a, b). MySQL renders "LIMIT a, b",
	// while PostgreSQL/SQLite render "LIMIT a OFFSET b".
	LimitOffset(a, b int) string
}

// Placeholders returns a comma-separated list of n placeholders for the
// dialect, starting at the given offset (1-indexed). For example, with n=3
// and offset=1: MySQL/SQLite → "?,?,?", PostgreSQL → "$1,$2,$3".
func (e Engine) Placeholders(n, offset int) string {
	d := e.dialect()
	b := make([]string, n)
	for i := range n {
		b[i] = d.Placeholder(offset + i)
	}
	return strings.Join(b, ",")
}

var dialects = map[Engine]Dialect{
	EngineMySQL:      mysqlDialect{},
	EnginePostgreSQL: postgresDialect{},
	EngineSQLite:     sqliteDialect{},
}

// RegisterDialect registers a [Dialect] for the given engine. This allows
// backend submodules to provide their own dialect implementations.
func RegisterDialect(e Engine, d Dialect) {
	dialects[e] = d
}

func (e Engine) dialect() Dialect {
	if d, ok := dialects[e]; ok {
		return d
	}
	return mysqlDialect{} // default fallback
}

// defaultExportArg handles shared export logic for types common to all engines.
func defaultExportArg(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case fmt.Stringer:
		return val.String()
	case driver.Valuer:
		return v
	default:
		rv := reflect.ValueOf(v)
		if rv.Type().Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil
			}
			return defaultExportArg(rv.Elem().Interface())
		}
		return v
	}
}

// mysqlDialect implements Dialect for MySQL/MariaDB.
type mysqlDialect struct{}

func (mysqlDialect) Placeholder(_ int) string { return "?" }

func (mysqlDialect) LimitOffset(a, b int) string {
	return "LIMIT " + strconv.Itoa(a) + ", " + strconv.Itoa(b)
}

func (mysqlDialect) ExportArg(v any) any {
	switch val := v.(type) {
	case time.Time:
		if val.IsZero() {
			return "0000-00-00 00:00:00.000000"
		}
		return val.UTC().Format("2006-01-02 15:04:05.999999")
	case *time.Time:
		if val == nil {
			return nil
		}
		return val.UTC().Format("2006-01-02 15:04:05.999999")
	}
	return defaultExportArg(v)
}

// postgresDialect implements Dialect for PostgreSQL/CockroachDB.
type postgresDialect struct{}

func (postgresDialect) Placeholder(n int) string {
	return "$" + strconv.Itoa(n)
}

func (postgresDialect) LimitOffset(a, b int) string {
	return "LIMIT " + strconv.Itoa(a) + " OFFSET " + strconv.Itoa(b)
}

func (postgresDialect) ExportArg(v any) any {
	switch val := v.(type) {
	case time.Time:
		if val.IsZero() {
			return "0001-01-01 00:00:00.000000"
		}
		return val.UTC().Format("2006-01-02 15:04:05.999999")
	case *time.Time:
		if val == nil {
			return nil
		}
		return val.UTC().Format("2006-01-02 15:04:05.999999")
	}
	return defaultExportArg(v)
}

// sqliteDialect implements Dialect for SQLite.
type sqliteDialect struct{}

func (sqliteDialect) Placeholder(_ int) string { return "?" }

func (sqliteDialect) LimitOffset(a, b int) string {
	return "LIMIT " + strconv.Itoa(a) + " OFFSET " + strconv.Itoa(b)
}

func (sqliteDialect) ExportArg(v any) any {
	switch val := v.(type) {
	case time.Time:
		if val.IsZero() {
			return "0001-01-01T00:00:00.000000Z"
		}
		return val.UTC().Format(time.RFC3339Nano)
	case *time.Time:
		if val == nil {
			return nil
		}
		return val.UTC().Format(time.RFC3339Nano)
	}
	return defaultExportArg(v)
}
