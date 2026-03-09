package psql

import (
	"context"
	"strings"
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

// SchemaChecker handles table structure verification and creation.
// Dialects that implement this interface will be called to check and create tables.
type SchemaChecker interface {
	CheckStructure(ctx context.Context, be *Backend, tv TableView) error
}

// TypeMapper handles engine-specific SQL type mapping and field definitions.
type TypeMapper interface {
	SqlType(baseType string, attrs map[string]string) string
	FieldDef(column, sqlType string, nullable bool, attrs map[string]string) string
	FieldDefAlter(column, sqlType string, nullable bool, attrs map[string]string) string
}

// KeyRenderer handles engine-specific key/index definitions.
type KeyRenderer interface {
	KeyDef(k *StructKey, tableName string) string
	InlineKeyDef(k *StructKey, tableName string) string // for CREATE TABLE
	CreateIndex(k *StructKey, tableName string) string  // for standalone CREATE INDEX
}

// UpsertRenderer handles engine-specific REPLACE and INSERT IGNORE syntax.
type UpsertRenderer interface {
	ReplaceSQL(tableName, fldStr, placeholders string, mainKey *StructKey, fields []*StructField) string
	InsertIgnoreSQL(tableName, fldStr, placeholders string) string
}

// ErrorClassifier handles engine-specific error interpretation.
type ErrorClassifier interface {
	ErrorNumber(err error) uint16
	IsNotExist(err error) bool
}

// BackendFactory creates backends from DSN strings.
type BackendFactory interface {
	MatchDSN(dsn string) bool
	CreateBackend(dsn string) (*Backend, error)
}

// VectorRenderer handles engine-specific vector distance syntax.
type VectorRenderer interface {
	VectorDistanceExpr(fieldExpr, vecExpr string, op VectorDistanceOp) string
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

var dialects = map[Engine]Dialect{}

var backendFactories []BackendFactory

// RegisterDialect registers a [Dialect] for the given engine. This allows
// backend submodules to provide their own dialect implementations.
func RegisterDialect(e Engine, d Dialect) {
	dialects[e] = d
}

// RegisterBackendFactory registers a [BackendFactory] that can create backends
// from DSN strings. Factories are tried in registration order by [New].
func RegisterBackendFactory(f BackendFactory) {
	backendFactories = append(backendFactories, f)
}

func (e Engine) dialect() Dialect {
	if d, ok := dialects[e]; ok {
		return d
	}
	return defaultDialect{} // minimal fallback
}

// defaultDialect provides a minimal fallback dialect when no engine-specific
// dialect has been registered. Uses ? placeholders and MySQL-like LIMIT syntax.
type defaultDialect struct{}

func (defaultDialect) Placeholder(_ int) string { return "?" }

func (defaultDialect) LimitOffset(a, b int) string {
	return "LIMIT " + intStr(a) + ", " + intStr(b)
}

func (defaultDialect) ExportArg(v any) any {
	return defaultExportArg(v)
}
