package psql

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Escape takes any value and transforms it into a string that can be included in a MySQL query
func Escape(val any) string {
	switch v := val.(type) {
	case EscapeValueable:
		return v.EscapeValue()
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'g', -1, 64)
	case bool:
		if v {
			return "TRUE"
		} else {
			return "FALSE"
		}
	case []byte:
		if v == nil {
			return "NULL"
		}
		if len(v) == 0 {
			return "x''"
		}
		return "x'" + hex.EncodeToString(v) + "'"
	case string:
		// We enforce NO_BACKSLASH_ESCAPES
		return "'" + strings.ReplaceAll(v, "'", "''") + "'"
	case time.Time:
		if v.IsZero() {
			return "'0000-00-00 00:00:00.000000'"
		}
		return v.UTC().Format("'2006-01-02 15:04:05.999999'")
	case driver.Valuer:
		sub, err := v.Value()
		if err != nil {
			// wut
			return ""
		}
		return Escape(sub)
	case fmt.Stringer:
		return v.String()
	default:
		rv := reflect.ValueOf(val)
		switch rv.Type().Kind() {
		case reflect.Bool:
			if rv.Bool() {
				return "TRUE"
			} else {
				return "FALSE"
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return strconv.FormatInt(rv.Int(), 10)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return strconv.FormatUint(rv.Uint(), 10)
		case reflect.Float32:
			return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
		case reflect.Float64:
			return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
		case reflect.Complex64:
			return strconv.FormatComplex(rv.Complex(), 'g', -1, 64)
		case reflect.Complex128:
			return strconv.FormatComplex(rv.Complex(), 'g', -1, 128)
		case reflect.String:
			return Escape(rv.String())
		// TODO: Array, Interface, Map, Slice, Struct
		case reflect.Ptr:
			if rv.IsNil() {
				return "NULL"
			}
			return Escape(rv.Elem().Interface())
		default:
			return fmt.Sprintf("%v", val)
		}
	}
}

func EscapeWhereSub(key string, val any) string {
	b := &bytes.Buffer{}
	b.WriteString(fieldName(key).EscapeValue())
	not := false
	if n, ok := val.(*Not); ok {
		not = true
		val = n.V
	}

	switch v := val.(type) {
	case *Like:
		// ignore Field
		if not {
			b.WriteString(" NOT")
		}
		b.WriteString(" LIKE ")
		b.WriteString(Escape(v.Like))
		b.WriteString(" ESCAPE '\\'")
		return b.String()
	default:
		if not {
			b.WriteString("!=")
		} else {
			b.WriteByte('=')
		}
		b.WriteString(Escape(val))
		return b.String()
	}
}

func EscapeWhere(val any, glue string) string {
	switch v := val.(type) {
	case map[string]any:
		// key = value
		b := &bytes.Buffer{}
		b.WriteByte('(')
		first := true
		for key, sub := range v {
			if first {
				first = false
			} else {
				b.WriteByte(' ')
				b.WriteString(glue)
				b.WriteByte(' ')
			}
			b.WriteString(EscapeWhereSub(key, sub))
		}
		b.WriteByte(')')
		return b.String()
	case []string:
		// V, V, V...
		b := &bytes.Buffer{}
		b.WriteByte('(')
		first := true
		for _, sub := range v {
			if first {
				first = false
			} else {
				b.WriteByte(' ')
				b.WriteString(glue)
				b.WriteByte(' ')
			}
			b.WriteString(EscapeWhere(sub, glue))
		}
		b.WriteByte(')')
		return b.String()
	case []any:
		// V, V, V...
		b := &bytes.Buffer{}
		b.WriteByte('(')
		first := true
		for _, sub := range v {
			if first {
				first = false
			} else {
				b.WriteByte(' ')
				b.WriteString(glue)
				b.WriteByte(' ')
			}
			b.WriteString(EscapeWhere(sub, glue))
		}
		b.WriteByte(')')
		return b.String()
	default:
		return Escape(val)
	}
}
