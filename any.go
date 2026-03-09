package psql

import (
	"bytes"
	"reflect"
)

// Any wraps a slice value for use with PostgreSQL's = ANY($1) syntax.
// On PostgreSQL with parameterized queries, it renders as = ANY($N) passing
// the slice as a single array parameter. On MySQL/SQLite (or non-parameterized),
// it expands to IN(v1,v2,...).
type Any struct {
	Values any // must be a slice
}

// escapeAnyInWhere renders the ANY/IN expression for a given field key.
func escapeAnyInWhere(ctx *renderContext, key string, v *Any, not bool) string {
	b := &bytes.Buffer{}
	b.WriteString(fieldName(key).EscapeValue())

	rv := reflect.ValueOf(v.Values)
	if rv.Kind() != reflect.Slice {
		// fallback: treat as equality
		if not {
			b.WriteString("!=")
		} else {
			b.WriteByte('=')
		}
		b.WriteString(escapeCtx(ctx, v.Values))
		return b.String()
	}

	if rv.Len() == 0 {
		return "FALSE"
	}

	// PostgreSQL with parameterized queries: use = ANY($N)
	if ctx != nil && ctx.useArgs && ctx.e == EnginePostgreSQL {
		if not {
			b.WriteString(" != ALL(")
		} else {
			b.WriteString(" = ANY(")
		}
		b.WriteString(ctx.appendArg(v.Values))
		b.WriteByte(')')
		return b.String()
	}

	// MySQL/SQLite or non-parameterized: expand to IN(...)
	if not {
		b.WriteString(" NOT IN(")
	} else {
		b.WriteString(" IN(")
	}
	for i := 0; i < rv.Len(); i++ {
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(escapeCtx(ctx, rv.Index(i).Interface()))
	}
	b.WriteByte(')')
	return b.String()
}
