// Package sqlite registers the SQLite dialect and backend factory for psql.
//
// Import this package with a blank identifier to enable SQLite support:
//
//	import _ "github.com/KarpelesLab/psql/sqlite"
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/KarpelesLab/psql"
	_ "modernc.org/sqlite"
)

func init() {
	psql.RegisterDialect(psql.EngineSQLite, sqliteDialect{})
	psql.RegisterBackendFactory(&sqliteFactory{})

	// Register engine-specific magic types
	psql.DefineMagicTypeEngine(psql.EngineSQLite, "DATETIME", "type=TEXT")
	psql.DefineMagicTypeEngine(psql.EngineSQLite, "JSON", "type=TEXT,format=json")
}

// sqliteDialect implements psql.Dialect and optional interfaces for SQLite.
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
	return psql.DefaultExportArg(v)
}

// TypeMapper implementation

func (sqliteDialect) SqlType(baseType string, attrs map[string]string) string {
	switch baseType {
	case "enum", "set":
		return "text"
	case "vector":
		return "text"
	default:
		return sqliteTypeAffinity(baseType)
	}
}

func (sqliteDialect) FieldDef(column, sqlType string, nullable bool, attrs map[string]string) string {
	mydef := psql.QuoteName(column) + " " + sqlType

	if null, ok := attrs["null"]; ok {
		switch null {
		case "0", "false":
			mydef += " NOT NULL"
		case "1", "true":
			mydef += " NULL"
		default:
			return ""
		}
	}
	if def, ok := attrs["default"]; ok {
		if def == "\\N" {
			mydef += " DEFAULT NULL"
		} else {
			mydef += " DEFAULT " + psql.Escape(def)
		}
	}

	// SQLite only supports BINARY, NOCASE, RTRIM collations — skip COLLATE

	return mydef
}

func (sqliteDialect) FieldDefAlter(column, sqlType string, nullable bool, attrs map[string]string) string {
	mydef := psql.QuoteName(column) + " " + sqlType

	hasDefault := false
	isNotNull := false

	if null, ok := attrs["null"]; ok {
		switch null {
		case "0", "false":
			isNotNull = true
		}
	}

	if def, ok := attrs["default"]; ok {
		hasDefault = true
		if def == "\\N" {
			mydef += " DEFAULT NULL"
		} else {
			mydef += " DEFAULT " + psql.Escape(def)
		}
	}

	// SQLite requires a DEFAULT for NOT NULL columns added via ALTER TABLE
	if isNotNull {
		if !hasDefault {
			switch sqlType {
			case "integer":
				mydef += " NOT NULL DEFAULT 0"
			case "real":
				mydef += " NOT NULL DEFAULT 0.0"
			default:
				mydef += " NOT NULL DEFAULT ''"
			}
		} else {
			mydef += " NOT NULL"
		}
	}

	return mydef
}

// KeyRenderer implementation

func (sqliteDialect) KeyDef(k *psql.StructKey, tableName string) string {
	return createIndexSQLite(k, tableName)
}

func (sqliteDialect) InlineKeyDef(k *psql.StructKey, tableName string) string {
	s := &strings.Builder{}
	switch k.Typ {
	case psql.KeyPrimary:
		s.WriteString("PRIMARY KEY (")
		for i, f := range k.Fields {
			if i > 0 {
				s.WriteString(", ")
			}
			s.WriteString(psql.QuoteName(f))
		}
		s.WriteByte(')')
		return s.String()
	case psql.KeyUnique:
		s.WriteString("UNIQUE (")
		for i, f := range k.Fields {
			if i > 0 {
				s.WriteString(", ")
			}
			s.WriteString(psql.QuoteName(f))
		}
		s.WriteByte(')')
		return s.String()
	default:
		return "" // non-inline indexes handled separately
	}
}

func (sqliteDialect) CreateIndex(k *psql.StructKey, tableName string) string {
	return createIndexSQLite(k, tableName)
}

func createIndexSQLite(k *psql.StructKey, tableName string) string {
	s := &strings.Builder{}

	switch k.Typ {
	case psql.KeyPrimary:
		return "" // handled inline
	case psql.KeyUnique:
		s.WriteString("CREATE UNIQUE INDEX ")
	case psql.KeyIndex:
		s.WriteString("CREATE INDEX ")
	default:
		// FULLTEXT, SPATIAL, VECTOR not supported in SQLite
		return ""
	}

	s.WriteString(psql.QuoteName(tableName + "_" + k.Key))
	s.WriteString(" ON ")
	s.WriteString(psql.QuoteName(tableName))
	s.WriteString(" (")
	for n, f := range k.Fields {
		if n > 0 {
			s.WriteString(", ")
		}
		s.WriteString(psql.QuoteName(f))
	}
	s.WriteByte(')')
	return s.String()
}

// UpsertRenderer implementation

func (sqliteDialect) ReplaceSQL(tableName, fldStr, placeholders string, mainKey *psql.StructKey, fields []*psql.StructField) string {
	return "INSERT OR REPLACE INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ")"
}

func (sqliteDialect) InsertIgnoreSQL(tableName, fldStr, placeholders string) string {
	return "INSERT OR IGNORE INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ")"
}

// DuplicateChecker implementation

func (sqliteDialect) IsDuplicate(err error) bool {
	for e := err; e != nil; {
		if strings.Contains(e.Error(), "UNIQUE constraint failed") {
			return true
		}
		if u, ok := e.(interface{ Unwrap() error }); ok {
			e = u.Unwrap()
		} else {
			break
		}
	}
	return false
}

// SchemaChecker implementation

func (sqliteDialect) CheckStructure(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	return checkStructureSQLite(ctx, be, tv)
}

// sqliteFactory implements psql.BackendFactory for SQLite DSNs.
type sqliteFactory struct{}

func (sqliteFactory) MatchDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "sqlite:") ||
		strings.HasPrefix(dsn, "file:") ||
		strings.HasSuffix(dsn, ".db") ||
		strings.HasSuffix(dsn, ".sqlite") ||
		strings.HasSuffix(dsn, ".sqlite3") ||
		dsn == ":memory:"
}

func (sqliteFactory) CreateBackend(dsn string) (*psql.Backend, error) {
	return New(strings.TrimPrefix(dsn, "sqlite:"))
}

// New creates a psql.Backend connected to a SQLite database at the given path.
// Pass ":memory:" for an in-memory database. WAL mode and foreign keys are
// enabled automatically.
func New(dsn string) (*psql.Backend, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite connection failed: %w", err)
	}

	// SQLite doesn't handle concurrent writes well
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	// Enable WAL mode and foreign keys
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		slog.Warn(fmt.Sprintf("[sqlite] failed to enable WAL mode: %s", err), "event", "psql:init:sqlite_wal")
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		slog.Warn(fmt.Sprintf("[sqlite] failed to enable foreign keys: %s", err), "event", "psql:init:sqlite_fk")
	}

	be := psql.NewBackend(psql.EngineSQLite, db)
	return be, nil
}

// sqliteTypeAffinity maps SQL types to SQLite type affinity names.
func sqliteTypeAffinity(typ string) string {
	typ = strings.ToLower(typ)
	switch typ {
	case "tinyint", "smallint", "mediumint", "int", "integer", "bigint", "boolean", "bool":
		return "integer"
	case "float", "double", "real", "double precision", "numeric", "decimal":
		return "real"
	case "blob", "binary", "varbinary", "longblob", "mediumblob", "tinyblob":
		return "blob"
	case "char", "varchar", "text", "longtext", "mediumtext", "tinytext",
		"timestamp", "datetime", "date", "time",
		"jsonb", "json", "uuid", "xml", "cidr", "inet":
		return "text"
	default:
		return "text"
	}
}

// checkStructureSQLite checks and creates the SQLite table structure.
func checkStructureSQLite(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	if v, ok := tv.TableAttrs()["check"]; ok && v == "0" {
		return nil
	}

	tableName := tv.FormattedName(be)

	var count int
	err := psql.Q("SELECT COUNT(1) FROM sqlite_master WHERE type='table' AND name=?", tableName).Each(ctx, func(rows *sql.Rows) error {
		return rows.Scan(&count)
	})
	if err != nil {
		return fmt.Errorf("while checking table existence: %w", err)
	}

	if count == 0 {
		return createTableSQLite(ctx, be, tv)
	}

	// Table exists, check columns
	type pragmaCol struct {
		CID        int
		Name       string
		Type       string
		NotNull    int
		DefaultVal *string
		PK         int
	}

	var existingCols []pragmaCol
	err = psql.Q(fmt.Sprintf("PRAGMA table_info(%s)", psql.QuoteName(tableName))).Each(ctx, func(rows *sql.Rows) error {
		var c pragmaCol
		if err := rows.Scan(&c.CID, &c.Name, &c.Type, &c.NotNull, &c.DefaultVal, &c.PK); err != nil {
			return err
		}
		existingCols = append(existingCols, c)
		return nil
	})
	if err != nil {
		return fmt.Errorf("while reading table info: %w", err)
	}

	colSet := make(map[string]bool)
	for _, c := range existingCols {
		colSet[c.Name] = true
	}

	for _, f := range tv.AllFields() {
		if colSet[f.Column] {
			continue
		}
		colDef := f.DefStringAlter(be)
		if colDef == "" {
			continue
		}
		alterSQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", psql.QuoteName(tableName), colDef)
		slog.Debug(fmt.Sprintf("[psql] SQLite ALTER: %s", alterSQL), "event", "psql:check:alter_sqlite", "table", tv.TableName())
		if err := psql.Q(alterSQL).Exec(ctx); err != nil {
			return fmt.Errorf("while adding column to %s: %w", tv.TableName(), err)
		}
	}

	// Check for missing indexes
	existingIdxs := make(map[string]bool)
	err = psql.Q(fmt.Sprintf("PRAGMA index_list(%s)", psql.QuoteName(tableName))).Each(ctx, func(rows *sql.Rows) error {
		var seq int
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return err
		}
		existingIdxs[name] = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("while reading index list: %w", err)
	}

	for _, k := range tv.AllKeys() {
		if k.Typ == psql.KeyPrimary {
			continue
		}
		idxName := tableName + "_" + k.Key
		if existingIdxs[idxName] {
			continue
		}
		createSQL := createIndexSQLite(k, tableName)
		if createSQL == "" {
			continue
		}
		slog.Debug(fmt.Sprintf("[psql] Creating SQLite index: %s", createSQL), "event", "psql:check:create_index_sqlite", "table", tv.TableName())
		if err := psql.Q(createSQL).Exec(ctx); err != nil {
			return fmt.Errorf("while creating index on %s: %w", tv.TableName(), err)
		}
	}

	return nil
}

func createTableSQLite(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	tableName := tv.FormattedName(be)

	sb := &strings.Builder{}
	sb.WriteString("CREATE TABLE ")
	sb.WriteString(psql.QuoteName(tableName))
	sb.WriteString(" (")

	for n, f := range tv.AllFields() {
		if n > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(f.DefString(be))
	}

	for _, k := range tv.AllKeys() {
		if len(k.Fields) == 0 {
			continue
		}
		if k.Typ == psql.KeyPrimary || k.Typ == psql.KeyUnique {
			sb.WriteString(", ")
			d := sqliteDialect{}
			sb.WriteString(d.InlineKeyDef(k, tableName))
		}
	}

	sb.WriteByte(')')

	if err := psql.Q(sb.String()).Exec(ctx); err != nil {
		return fmt.Errorf("while creating table: %w", err)
	}

	for _, k := range tv.AllKeys() {
		if len(k.Fields) == 0 || k.Typ == psql.KeyPrimary || k.Typ == psql.KeyUnique {
			continue
		}
		createSQL := createIndexSQLite(k, tableName)
		if createSQL == "" {
			continue
		}
		if err := psql.Q(createSQL).Exec(ctx); err != nil {
			return fmt.Errorf("while creating index: %w", err)
		}
	}

	return nil
}
