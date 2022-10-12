package psql

import "strings"

const (
	// Use " for ANSI SQL, and ` for MySQL's own thing
	NameQuoteChar = `"`
	NameQuoteRune = '"'
)

type FieldName string

type FullField struct {
	TableName
	FieldName
}

func (f FieldName) String() string {
	return string(f)
}

func (f FieldName) EscapeValue() string {
	if f == "*" {
		// special case
		return "*"
	}
	// we consider table names won't contain dots, if it do use FullField instead of FieldName
	return NameQuoteChar + strings.Replace(strings.ReplaceAll(string(f), NameQuoteChar, NameQuoteChar+NameQuoteChar), ".", NameQuoteChar+"."+NameQuoteChar, 1) + NameQuoteChar
}

func (f *FullField) String() string {
	return f.EscapeValue()
}

func (f *FullField) EscapeValue() string {
	if f.TableName == "" {
		return QuoteName(string(f.FieldName))
	}
	return NameQuoteChar + strings.ReplaceAll(string(f.TableName), NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar + "." + NameQuoteChar + strings.ReplaceAll(string(f.FieldName), NameQuoteChar, NameQuoteChar+NameQuoteChar) + NameQuoteChar
}

type TableName string

func (t TableName) String() string {
	return string(t)
}

func (t TableName) EscapeTable() string {
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
