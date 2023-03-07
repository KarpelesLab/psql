package psql

import (
	"strings"
)

type WhereAND []any
type WhereOR []any

func (w WhereAND) String() string {
	return w.EscapeValue()
}

func (w WhereAND) EscapeValue() string {
	return w.escapeValueCtx(nil)
}

func (w WhereAND) escapeValueCtx(ctx *renderContext) string {
	b := &strings.Builder{}

	for n, v := range w {
		if n > 0 {
			b.WriteString(" AND ")
		}
		b.WriteByte('(')
		b.WriteString(escapeWhere(ctx, v, " AND "))
		b.WriteByte(')')
	}

	return b.String()
}

func (w WhereOR) String() string {
	return w.EscapeValue()
}

func (w WhereOR) EscapeValue() string {
	return w.escapeValueCtx(nil)
}

func (w WhereOR) escapeValueCtx(ctx *renderContext) string {
	b := &strings.Builder{}

	for n, v := range w {
		if n > 0 {
			b.WriteString(" OR ")
		}
		b.WriteByte('(')
		b.WriteString(escapeWhere(ctx, v, " OR "))
		b.WriteByte(')')
	}

	return b.String()
}
