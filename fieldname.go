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

type fieldName string

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

func (f *fullField) String() string {
	return f.EscapeValue()
}

func (f *fullField) EscapeValue() string {
	if f.tableName == "" {
		return QuoteName(string(f.fieldName))
	}
	return NameQuoteChar + strings.ReplaceAll(string(f.tableName), NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar + "." + NameQuoteChar + strings.ReplaceAll(string(f.fieldName), NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar
}

type tableName string

func (t tableName) String() string {
	return string(t)
}

func (t tableName) EscapeTable() string {
	return QuoteName(string(t))
}

// quote a name (field, etc)
func QuoteName(v string) string {
	pos := strings.IndexByte(v, NameQuoteRune)
	if pos == -1 {
		return NameQuoteChar + v + NameQuoteChar
	} else {
		return NameQuoteChar + strings.ReplaceAll(v, NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar
	}
}
