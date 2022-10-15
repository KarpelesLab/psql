package psql

// Starting MySQL 8.0.19, numeric types length specification has been deprecated and may not be reflected, except for tinyint(1) or zerofill stuff.
// Because we don't want to depend on the version we instead detect if a type is numeric, in which case we ignore the length difference
var numericTypes = map[string]bool{
	"bit":              true,
	"tinyint":          true,
	"tinyint(1)":       false, // exception
	"smallint":         true,
	"mediumint":        true,
	"int":              true,
	"integer":          true,
	"bigint":           true,
	"float":            true,
	"double":           true,
	"double precision": true,
}

type ShowIndexResult struct {
	Virtual     Name   `sql:",check=0"`
	Table       string `sql:",type=VARCHAR,size=256"`
	NonUnique   bool   `sql:"Non_unique"`
	KeyName     string `sql:"Key_name,type=VARCHAR,size=256"`
	SeqInIndex  int64  `sql:"Seq_in_index"`
	ColumnName  string `sql:"Column_name,type=VARCHAR,size=256"`
	Collation   string `sql:",type=VARCHAR,size=256"`
	Cardinality int64
	SubPart     *int64 `sql:"Sub_part"`
	// Packed?
	Null         string `sql:",type=VARCHAR,size=3"`             // "YES" or ""
	IndexType    string `sql:"Index_type,type=VARCHAR,size=256"` // BTREE, HASH
	Comment      string `sql:",type=VARCHAR,size=256"`
	IndexComment string `sql:"Index_comment,type=VARCHAR,size=256"`
}
