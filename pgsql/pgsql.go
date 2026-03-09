// Package pgsql registers the PostgreSQL/CockroachDB dialect and backend factory for psql.
//
// Import this package with a blank identifier to enable PostgreSQL support:
//
//	import _ "github.com/KarpelesLab/psql/pgsql"
package pgsql

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/KarpelesLab/psql"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
)

func init() {
	psql.RegisterDialect(psql.EnginePostgreSQL, postgresDialect{})
	psql.RegisterBackendFactory(&pgsqlFactory{})

	// Register engine-specific magic types
	psql.DefineMagicTypeEngine(psql.EnginePostgreSQL, "DATETIME", "type=TIMESTAMP,size=6,default='1970-01-01 00:00:00.000000'")
	psql.DefineMagicTypeEngine(psql.EnginePostgreSQL, "JSON", "type=JSONB,format=json")
}

// numericTypesNoLength lists PostgreSQL numeric types that should not have a length specification.
var numericTypesNoLength = map[string]bool{
	"bit": true, "tinyint": true, "smallint": true, "mediumint": true,
	"int": true, "integer": true, "bigint": true,
	"float": true, "double": true, "double precision": true,
}

// postgresDialect implements psql.Dialect and optional interfaces for PostgreSQL.
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
	return psql.DefaultExportArg(v)
}

// TypeMapper implementation

func (postgresDialect) SqlType(baseType string, attrs map[string]string) string {
	switch baseType {
	case "enum":
		if mysize, ok := attrs["size"]; ok {
			return "varchar(" + mysize + ")"
		}
		return "varchar(64)"
	case "set":
		return "varchar(128)"
	case "vector":
		if mysize, ok := attrs["size"]; ok {
			return "vector(" + mysize + ")"
		}
		return "vector"
	default:
		// PostgreSQL numeric types should have no length
		if numericTypesNoLength[baseType] {
			return baseType
		}
		if mysize, ok := attrs["size"]; ok {
			return baseType + "(" + mysize + ")"
		}
		return baseType
	}
}

func (postgresDialect) FieldDef(column, sqlType string, nullable bool, attrs map[string]string) string {
	setType := false
	if sqlType == "set" {
		sqlType = "jsonb"
		setType = true
	}

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
		if setType {
			js, _ := json.Marshal([]string{def})
			def = string(js)
		}
		if def == "\\N" {
			mydef += " DEFAULT NULL"
		} else {
			mydef += " DEFAULT " + psql.Escape(def)
		}
	}

	// PostgreSQL supports COLLATE but we skip it for simplicity
	// (collation handling differs significantly from MySQL)

	return mydef
}

func (d postgresDialect) FieldDefAlter(column, sqlType string, nullable bool, attrs map[string]string) string {
	return d.FieldDef(column, sqlType, nullable, attrs)
}

// KeyRenderer implementation

func (postgresDialect) KeyDef(k *psql.StructKey, tableName string) string {
	// For ALTER TABLE ADD, use CREATE INDEX style
	return pgCreateIndex(k, tableName)
}

func (postgresDialect) InlineKeyDef(k *psql.StructKey, tableName string) string {
	return pgInlineKeyDef(k, tableName)
}

func (postgresDialect) CreateIndex(k *psql.StructKey, tableName string) string {
	return pgCreateIndex(k, tableName)
}

func pgKeyName(k *psql.StructKey, tableName string) string {
	return tableName + "_" + k.Key
}

func pgInlineKeyDef(k *psql.StructKey, tableName string) string {
	s := &strings.Builder{}

	switch k.Typ {
	case psql.KeyPrimary:
		s.WriteString("PRIMARY KEY ")
	case psql.KeyUnique:
		s.WriteString("CONSTRAINT ")
		s.WriteString(psql.QuoteName(pgKeyName(k, tableName)))
		s.WriteString(" UNIQUE ")
	default:
		return "" // non-inline indexes handled separately
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

func pgCreateIndex(k *psql.StructKey, tableName string) string {
	s := &strings.Builder{}

	switch k.Typ {
	case psql.KeyPrimary:
		return "" // created inline
	case psql.KeyUnique:
		s.WriteString("CREATE UNIQUE INDEX ")
		s.WriteString(psql.QuoteName(pgKeyName(k, tableName)))
	case psql.KeyIndex:
		s.WriteString("CREATE INDEX ")
		s.WriteString(psql.QuoteName(pgKeyName(k, tableName)))
	case psql.KeyVector:
		s.WriteString("CREATE INDEX ")
		s.WriteString(psql.QuoteName(pgKeyName(k, tableName)))
	default:
		return "" // FULLTEXT and SPATIAL not supported
	}

	s.WriteString(" ON ")
	s.WriteString(psql.QuoteName(tableName))

	if k.Typ == psql.KeyVector {
		method := "hnsw"
		if m, ok := k.Attrs["method"]; ok {
			method = strings.ToLower(m)
		}
		s.WriteString(" USING ")
		s.WriteString(method)
	}

	s.WriteString(" (")
	for n, f := range k.Fields {
		if n > 0 {
			s.WriteString(", ")
		}
		s.WriteString(psql.QuoteName(f))
		if k.Typ == psql.KeyVector {
			if opclass, ok := k.Attrs["opclass"]; ok {
				s.WriteString(" ")
				s.WriteString(opclass)
			}
		}
	}
	s.WriteByte(')')
	return s.String()
}

// UpsertRenderer implementation

func (postgresDialect) ReplaceSQL(tableName, fldStr, placeholders string, mainKey *psql.StructKey, fields []*psql.StructField) string {
	if mainKey == nil {
		// Cannot use upsert without a key on PG
		return "INSERT INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ")"
	}

	req := "INSERT INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ") ON CONFLICT ("
	for i, col := range mainKey.Fields {
		if i > 0 {
			req += ","
		}
		req += psql.QuoteName(col)
	}
	req += ") DO UPDATE SET "
	first := true
	keyFields := make(map[string]bool)
	for _, col := range mainKey.Fields {
		keyFields[col] = true
	}
	for _, f := range fields {
		if keyFields[f.Column] {
			continue
		}
		if !first {
			req += ","
		}
		first = false
		req += psql.QuoteName(f.Column) + "=EXCLUDED." + psql.QuoteName(f.Column)
	}
	return req
}

func (postgresDialect) InsertIgnoreSQL(tableName, fldStr, placeholders string) string {
	return "INSERT INTO " + psql.QuoteName(tableName) + " (" + fldStr + ") VALUES (" + placeholders + ") ON CONFLICT DO NOTHING"
}

// ErrorClassifier implementation

func (postgresDialect) ErrorNumber(err error) uint16 {
	return 0xffff // PG uses string codes, not numeric; handled by IsNotExist
}

func (postgresDialect) IsNotExist(err error) bool {
	// PG errors are handled by the core's fs.ErrNotExist check
	return false
}

// VectorRenderer implementation

func (postgresDialect) VectorDistanceExpr(fieldExpr, vecExpr string, op psql.VectorDistanceOp) string {
	var opStr string
	switch op {
	case psql.VectorL2:
		opStr = " <-> "
	case psql.VectorCosine:
		opStr = " <=> "
	case psql.VectorInnerProduct:
		opStr = " <#> "
	default:
		opStr = " <-> "
	}
	return fieldExpr + opStr + vecExpr
}

// SchemaChecker implementation

func (postgresDialect) CheckStructure(ctx context.Context, be *psql.Backend, tv psql.TableView) error {
	return checkStructurePG(ctx, be, tv)
}

// pgsqlFactory implements psql.BackendFactory for PostgreSQL DSNs.
type pgsqlFactory struct{}

func (pgsqlFactory) MatchDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgresql://") || strings.HasPrefix(dsn, "postgres://")
}

func (pgsqlFactory) CreateBackend(dsn string) (*psql.Backend, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	return New(cfg)
}

// New creates a psql.Backend connected to a PostgreSQL (or CockroachDB) database
// using the given pgxpool.Config.
func New(cfg *pgxpool.Config) (*psql.Backend, error) {
	pgdb, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}

	db := stdlib.OpenDBFromPool(pgdb)

	be := psql.NewBackend(psql.EnginePostgreSQL, db,
		psql.WithDriverData(pgdb),
		psql.WithPoolDefaults,
	)
	return be, nil
}

// PGYesOrNo represents a PostgreSQL yes_or_no type from information_schema.
type PGYesOrNo string

// V returns true if the value is "YES".
func (p PGYesOrNo) V() bool {
	return p == "YES"
}

// pgSchemaTables represents a row from information_schema.tables.
type pgSchemaTables struct {
	psql.Name `sql:",check=0"`
	Catalog   string `sql:"table_catalog,type=sql_identifier"`
	Schema    string `sql:"table_schema,type=sql_identifier"`
	Table     string `sql:"table_name,type=sql_identifier"`
	TableType string `sql:"table_type,type=character_data"`
}

// pgSchemaColumns represents a row from information_schema.columns.
type pgSchemaColumns struct {
	psql.Name      `sql:",check=0"`
	Catalog        string    `sql:"table_catalog,type=sql_identifier"`
	Schema         string    `sql:"table_schema,type=sql_identifier"`
	Table          string    `sql:"table_name,type=sql_identifier"`
	Column         string    `sql:"column_name,type=sql_identifier"`
	OrdinalPos     uint      `sql:"ordinal_position,type=cardinal_number"`
	Default        *string   `sql:"column_default,type=character_data"`
	IsNullable     PGYesOrNo `sql:"is_nullable,type=yes_or_no"`
	DataType       string    `sql:"data_type,type=character_data"`
	MaxLen         *uint     `sql:"character_maximum_length,type=cardinal_number"`
	MaxOctetLen    *uint     `sql:"character_octet_length,type=cardinal_number"`
	Precision      *uint     `sql:"numeric_precision,type=cardinal_number"`
	PrecisionRadix *uint     `sql:"numeric_precision_radix,type=cardinal_number"`
	NumericScale   *uint     `sql:"numeric_scale,type=cardinal_number"`
	DatetimePrec   *uint     `sql:"datetime_precision,type=cardinal_number"`
	IntervalType   *string   `sql:"interval_type,type=character_data"`
	IntervalPrec   *uint     `sql:"interval_precision,type=cardinal_number"`
	CharsetCatalog *string   `sql:"character_set_catalog,type=sql_identifier"`
	CharsetSchema  *string   `sql:"character_set_schema,type=sql_identifier"`
	CharsetName    *string   `sql:"character_set_name,type=sql_identifier"`
}
