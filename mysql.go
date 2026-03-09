package psql

// NumericTypes lists MySQL numeric types and whether their length specification
// can be ignored for comparison purposes. Starting MySQL 8.0.19, numeric types
// length specification has been deprecated. The bool value indicates whether the
// length should be ignored (true) or is significant (false, e.g., tinyint(1)).
var NumericTypes = map[string]bool{
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
