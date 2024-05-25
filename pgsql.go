package psql

// various pg-specific schemas
// see: https://www.postgresql.org/docs/current/information-schema.html

// pgSchemaColumns
// See: https://www.postgresql.org/docs/current/infoschema-columns.html
type pgSchemaColumns struct {
	Virtual        Name      `sql:",check=0"`
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

// pgSchemaTables
// See: https://www.postgresql.org/docs/current/infoschema-tables.html
type pgSchemaTables struct {
	Virtual   Name   `sql:",check=0"`
	Catalog   string `sql:"table_catalog,type=sql_identifier"`
	Schema    string `sql:"table_schema,type=sql_identifier"`
	Table     string `sql:"table_name,type=sql_identifier"`
	TableType string `sql:"table_type,type=character_data"` // BASE TABLE | VIEW | FOREIGN | LOCAL TEMPORARY
}

type PGYesOrNo string

func (p PGYesOrNo) V() bool {
	return p == "YES"
}

type pgShowIndex struct {
	Virtual    Name   `sql:",check=0"`
	Table      string `sql:"table_name,type=sql_identifier"`
	Index      string `sql:"index_name,type=sql_identifier"`
	NonUnique  string `sql:"non_unique,type=character_data"` // true|false
	SeqInIndex uint   `sql:"seq_in_index,type=cardinal_number"`
	Column     string `sql:"column_name,type=sql_identifier"`
	Direction  string `sql:"direction,type=character_data"` // ASC|DESC
	Storing    string `sql:"storing,type=character_data"`   // true|false
	Implicit   string `sql:"implicit,type=character_data"`
	Visible    string `sql:"visible,type=character_data"`
}
