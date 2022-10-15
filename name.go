package psql

import "reflect"

var nameType = reflect.TypeOf(Name{})

// Name allows specifying the table name when associating a table with a struct
//
// For example:
// type X struct {
// TableName psql.Name `sql:"X"`
// ...
// }
type Name struct{}
