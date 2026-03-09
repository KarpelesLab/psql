// Package mysql registers the MySQL/MariaDB dialect and backend factory for psql.
//
// Import this package with a blank identifier to enable MySQL support:
//
//	import _ "github.com/KarpelesLab/psql/mysql"
package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/KarpelesLab/psql"
	"github.com/go-sql-driver/mysql"
)

func init() {
	psql.RegisterDialect(psql.EngineMySQL, mysqlDialect{})
	psql.RegisterBackendFactory(&mysqlFactory{})

	// Register engine-specific magic types
	psql.DefineMagicTypeEngine(psql.EngineMySQL, "DATETIME", "type=DATETIME,size=6")
	psql.DefineMagicTypeEngine(psql.EngineMySQL, "JSON", "type=LONGTEXT,format=json")
}

// mysqlDialect implements psql.Dialect and optional interfaces for MySQL.
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
	return psql.DefaultExportArg(v)
}

// TypeMapper implementation

func (mysqlDialect) SqlType(baseType string, attrs map[string]string) string {
	switch baseType {
	case "enum", "set":
		if myvals, ok := attrs["values"]; ok {
			l := strings.Split(myvals, ",")
			return baseType + "('" + strings.Join(l, "','") + "')"
		}
		return ""
	case "vector":
		if mysize, ok := attrs["size"]; ok {
			return "vector(" + mysize + ")"
		}
		return "vector"
	default:
		if mysize, ok := attrs["size"]; ok {
			return baseType + "(" + mysize + ")"
		}
		return baseType
	}
}

func (mysqlDialect) FieldDef(column, sqlType string, nullable bool, attrs map[string]string) string {
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

	if mycol, ok := attrs["collation"]; ok {
		mydef += " COLLATE " + mycol
	}

	return mydef
}

func (d mysqlDialect) FieldDefAlter(column, sqlType string, nullable bool, attrs map[string]string) string {
	return d.FieldDef(column, sqlType, nullable, attrs)
}

// KeyRenderer implementation

func (mysqlDialect) KeyDef(k *psql.StructKey, tableName string) string {
	return inlineKeyDef(k)
}

func (mysqlDialect) InlineKeyDef(k *psql.StructKey, tableName string) string {
	return inlineKeyDef(k)
}

func (mysqlDialect) CreateIndex(k *psql.StructKey, tableName string) string {
	return "" // MySQL creates all keys inline
}

func inlineKeyDef(k *psql.StructKey) string {
	s := &strings.Builder{}

	switch k.Typ {
	case psql.KeyPrimary:
		s.WriteString("PRIMARY KEY ")
	case psql.KeyUnique:
		s.WriteString("UNIQUE INDEX ")
		s.WriteString(psql.QuoteName(k.Key))
	case psql.KeyIndex:
		s.WriteString("INDEX ")
		s.WriteString(psql.QuoteName(k.Key))
	case psql.KeyFulltext:
		s.WriteString("FULLTEXT INDEX ")
		s.WriteString(psql.QuoteName(k.Key))
	case psql.KeySpatial:
		s.WriteString("SPATIAL INDEX ")
		s.WriteString(psql.QuoteName(k.Key))
	case psql.KeyVector:
		return ""
	default:
		return ""
	}

	s.WriteByte('(')
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

func (mysqlDialect) ReplaceSQL(tableName, fldStr, placeholders string, mainKey *psql.StructKey, fields []*psql.StructField) string {
	return "REPLACE INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ")"
}

func (mysqlDialect) InsertIgnoreSQL(tableName, fldStr, placeholders string) string {
	return "INSERT IGNORE INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ")"
}

// ErrorClassifier implementation

func (mysqlDialect) ErrorNumber(err error) uint16 {
	for {
		if err == nil {
			return 0
		}
		switch e := err.(type) {
		case *mysql.MySQLError:
			return e.Number
		case interface{ Unwrap() error }:
			err = e.Unwrap()
		default:
			return 0xffff
		}
	}
}

func (mysqlDialect) IsNotExist(err error) bool {
	return false // handled by core via ErrorNumber
}

// SchemaChecker implementation

func (mysqlDialect) CheckStructure(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	return checkStructureMySQL(ctx, be, tv)
}

// mysqlFactory implements psql.BackendFactory for MySQL DSNs.
type mysqlFactory struct{}

func (mysqlFactory) MatchDSN(dsn string) bool {
	// MySQL DSNs are the fallback — they match anything that isn't obviously PG or SQLite
	_, err := mysql.ParseDSN(dsn)
	return err == nil
}

func (mysqlFactory) CreateBackend(dsn string) (*psql.Backend, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return New(cfg)
}

// New creates a psql.Backend connected to a MySQL database using the given
// mysql.Config. It sets ANSI SQL mode with NO_BACKSLASH_ESCAPES and configures
// connection pooling.
func New(cfg *mysql.Config) (*psql.Backend, error) {
	cfg.Params = map[string]string{
		"charset":  "utf8mb4",
		"sql_mode": "'ANSI,NO_BACKSLASH_ESCAPES'",
	}

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}

	res, err := db.Query("SHOW VARIABLES LIKE 'version%'")
	if err != nil {
		return nil, fmt.Errorf("SHOW VARIABLES failed: %w", err)
	}

	defer res.Close()
	for res.Next() {
		var k, v string
		if err := res.Scan(&k, &v); err != nil {
			panic(err)
		}
		slog.Debug(fmt.Sprintf("[mysql] %s = %s", k, v), "event", "psql:init:dbvar", "psql.dbvar", k)
	}

	be := psql.NewBackend(psql.EngineMySQL, db, psql.WithPoolDefaults)
	return be, nil
}

// InitCfg creates a new MySQL Backend from the given config and sets it as psql.DefaultBackend.
func InitCfg(cfg *mysql.Config) error {
	be, err := New(cfg)
	if err != nil {
		return err
	}
	psql.DefaultBackend = be
	return nil
}
