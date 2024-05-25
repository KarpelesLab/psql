package psql

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/KarpelesLab/typutil"
)

// Escape takes any value and transforms it into a string that can be included in a MySQL query
func Escape(val any) string {
	return escapeCtx(nil, val)
}

func escapeCtx(ctx *renderContext, val any) string {
	if ctx != nil && ctx.useArgs {
		switch v := val.(type) {
		case *fullField, fieldName, tableName:
			break // contnue below
		case escapeValueCtxable:
			return v.escapeValueCtx(ctx)
		case *rawValue:
			return v.V
		default:
			return ctx.appendArg(val)
		}
	}
	// null check
	if val == nil {
		return "NULL"
	}

	switch v := val.(type) {
	case escapeValueCtxable:
		return v.escapeValueCtx(ctx)
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
		return escapeCtx(ctx, sub)
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
			return escapeCtx(ctx, rv.String())
		// TODO: Array, Interface, Map, Slice, Struct
		case reflect.Ptr:
			if rv.IsNil() {
				return "NULL"
			}
			return escapeCtx(ctx, rv.Elem().Interface())
		default:
			return fmt.Sprintf("%v", val)
		}
	}
}

func escapeWhereSub(ctx *renderContext, key string, val any) string {
	b := &bytes.Buffer{}
	b.WriteString(fieldName(key).EscapeValue())
	not := false
	if n, ok := val.(*Not); ok {
		not = true
		val = n.V
	}

	if typutil.IsNil(val) {
		if not {
			b.WriteString(" IS NOT NULL")
		} else {
			b.WriteString(" IS NULL")
		}
		return b.String()
	}

	switch v := typutil.Flatten(val).(type) {
	case nil:
		if not {
			b.WriteString(" IS NOT NULL")
		} else {
			b.WriteString(" IS NULL")
		}
		return b.String()
	case Like:
		// ignore Field
		if not {
			b.WriteString(" NOT")
		}
		b.WriteString(" LIKE ")
		b.WriteString(escapeCtx(ctx, v.Like))
		b.WriteString(" ESCAPE '\\'")
		return b.String()
	case FindInSet:
		// ignore Field
		b = &bytes.Buffer{}
		if not {
			b.WriteString("NOT ")
		}
		b.WriteString("FIND_IN_SET(")
		b.WriteString(escapeCtx(ctx, v.Value))
		b.WriteString(",")
		b.WriteString(fieldName(key).EscapeValue())
		b.WriteString(")")
		return b.String()
	case Comparison:
		// ignore Field (A) and only use B + Op
		b.WriteString(" " + v.opStr(not) + " ")
		b.WriteString(escapeCtx(ctx, v.B))
		return b.String()
	case betweenComp:
		// ignore Field (a) and only use start + end
		if not {
			b.WriteString(" NOT")
		}
		b.WriteString(" BETWEEN ")
		b.WriteString(escapeCtx(ctx, v.start))
		b.WriteString(" AND ")
		b.WriteString(escapeCtx(ctx, v.end))
		return b.String()
	case WhereOR:
		// need to build a OR and repeat the field for each possible option
		b = &bytes.Buffer{} // reset buffer
		b.WriteString("(")
		for n, subv := range v {
			if n > 0 {
				b.WriteString(" OR ")
			}
			// escape
			b.WriteString(escapeWhereSub(ctx, key, subv))
		}
		b.WriteString(")")
		return b.String()
	case WhereAND:
		// need to build a AND and repeat the field for each possible option
		b = &bytes.Buffer{} // reset buffer
		b.WriteString("(")
		for n, subv := range v {
			if n > 0 {
				b.WriteString(" AND ")
			}
			// escape
			b.WriteString(escapeWhereSub(ctx, key, subv))
		}
		b.WriteString(")")
		return b.String()
	case []any:
		// (in)
		if len(v) == 0 {
			// "column" IN (nothing) is always false
			return "FALSE"
		}
		if not {
			b.WriteString(" NOT IN(")
		} else {
			b.WriteString(" IN(")
		}
		for n, sub := range v {
			if n != 0 {
				b.WriteByte(',')
			}
			b.WriteString(escapeCtx(ctx, sub))
		}
		b.WriteByte(')')
		return b.String()
	case []string:
		// (in)
		if len(v) == 0 {
			// "column" IN (nothing) is always false
			return "FALSE"
		}
		if not {
			b.WriteString(" NOT IN(")
		} else {
			b.WriteString(" IN(")
		}
		for n, sub := range v {
			if n != 0 {
				b.WriteByte(',')
			}
			b.WriteString(escapeCtx(ctx, sub))
		}
		b.WriteByte(')')
		return b.String()
	case map[string]any:
		// can be a number of things depending on the key, and can be multiple conditions that add as a AND
		var conds []string
		for k, s := range v {
			switch k {
			case "$gt":
				conds = append(conds, fieldName(key).EscapeValue()+">"+escapeCtx(ctx, s))
			case "$lt":
				conds = append(conds, fieldName(key).EscapeValue()+"<"+escapeCtx(ctx, s))
			case "$gte":
				conds = append(conds, fieldName(key).EscapeValue()+">="+escapeCtx(ctx, s))
			case "$lte":
				conds = append(conds, fieldName(key).EscapeValue()+"<="+escapeCtx(ctx, s))
			}
		}

		if len(conds) == 1 {
			return conds[0]
		} else if len(conds) == 0 {
			return "FALSE"
		} else {
			return "(" + strings.Join(conds, " AND ") + ")"
		}
	default:
		if not {
			b.WriteString("!=")
		} else {
			b.WriteByte('=')
		}
		b.WriteString(escapeCtx(ctx, val))
		return b.String()
	}
}

func escapeWhere(ctx *renderContext, val any, glue string) string {
	switch v := val.(type) {
	case map[string]any:
		if len(v) == 0 {
			// empty where → match all
			return "1"
		}
		// key = value
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		b := &bytes.Buffer{}
		first := true
		for _, key := range keys {
			sub := v[key]
			if first {
				first = false
			} else {
				b.WriteString(glue)
			}
			b.WriteString(escapeWhereSub(ctx, key, sub))
		}
		return b.String()
	case []string:
		if len(v) == 0 {
			// empty where → match all
			return "1"
		}
		// V, V, V...
		b := &bytes.Buffer{}
		first := true
		for _, sub := range v {
			if first {
				first = false
			} else {
				b.WriteString(glue)
			}
			b.WriteString(escapeWhere(ctx, sub, glue))
		}
		return b.String()
	case []any:
		if len(v) == 0 {
			// empty where → match all
			return "1"
		}
		// V, V, V...
		b := &bytes.Buffer{}
		first := true
		for _, sub := range v {
			if first {
				first = false
			} else {
				b.WriteString(glue)
			}
			b.WriteString(escapeWhere(ctx, sub, glue))
		}
		return b.String()
	default:
		return escapeCtx(ctx, val)
	}
}
