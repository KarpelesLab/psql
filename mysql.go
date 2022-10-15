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
