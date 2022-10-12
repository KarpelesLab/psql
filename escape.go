package psql

import (
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
