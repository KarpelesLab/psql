package psql

import (
	"fmt"
	"strings"
)

const (
	// Use " for ANSI SQL, and ` for MySQL's own thing
	NameQuoteChar = `"`
	NameQuoteRune = '"'
)

// F allows passing a field name to the query builder. It can be used in multiple ways:
//
// psql.F("field")
// psql.F("table.field")
// psql.F("", "field.with.dots")
// psql.F("table", "field")
// psql.F("table.with.dots", "field.with.dots")
// and more...
func F(field ...string) EscapeValueable {
	switch len(field) {
	case 1:
		return fieldName(field[0])
	case 2:
		return &fullField{tableName(field[0]), fieldName(field[1])}
	default:
		panic(fmt.Sprintf("psql.F() expects only one or two args, got %d", len(field)))
	}
}

func S(field ...string) SortValueable {
	// same as F but last value must be "ASC" or "DESC"
	if len(field) == 0 {
		panic("psql.S() expects at least one arg, got none")
	}
	last := field[len(field)-1]
	switch last {
	case "ASC", "DESC":
		return &ordField{ord: last, fld: F(field[:len(field)-1]...)}
	default:
		return &ordField{ord: "", fld: F(field...)}
	}
}

type fieldName string

type ordField struct {
	ord string // "ASC" or "DESC"
	fld EscapeValueable
}

func (f *ordField) sortEscapeValue() string {
	if f.ord == "" {
		return f.fld.EscapeValue()
	}
	return f.fld.EscapeValue() + " " + f.ord
}

type fullField struct {
	tableName
	fieldName
}

func (f fieldName) String() string {
	return string(f)
}

func (f fieldName) EscapeValue() string {
	if f == "*" {
		// special case
		return "*"
	}
	// we consider table names won't contain dots, if it do use fullField instead of fieldName
	return NameQuoteChar + strings.Replace(strings.ReplaceAll(string(f), NameQuoteChar, NameQuoteChar+NameQuoteChar), ".", NameQuoteChar+"."+NameQuoteChar, 1) + NameQuoteChar
}

func (f fieldName) sortEscapeValue() string {
	return f.EscapeValue()
}

func (f *fullField) String() string {
	return f.EscapeValue()
}

func (f *fullField) EscapeValue() string {
	if f.tableName == "" {
		return QuoteName(string(f.fieldName))
	}
	return NameQuoteChar + strings.ReplaceAll(string(f.tableName), NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar + "." + NameQuoteChar + strings.ReplaceAll(string(f.fieldName), NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar
}

func (f *fullField) sortEscapeValue() string {
	return f.EscapeValue()
}

type tableName string

func (t tableName) String() string {
	return string(t)
}

func (t tableName) EscapeTable() string {
	return QuoteName(string(t))
}

// quote a name (field, etc)
// This doesn't use the Namer since this is just for escaping a name that's already been formatted
func QuoteName(v string) string {
	pos := strings.IndexByte(v, NameQuoteRune)
	if pos == -1 {
		return NameQuoteChar + v + NameQuoteChar
	} else {
		return NameQuoteChar + strings.ReplaceAll(v, NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar
	}
}
