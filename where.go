package psql

import "strings"

type WhereAND []any
type WhereOR []any

func (w WhereAND) String() string {
	return w.EscapeValue()
}

func (w WhereAND) EscapeValue() string {
	b := &strings.Builder{}

	for n, v := range w {
		if n > 0 {
			b.WriteString(" AND ")
		}
		b.WriteByte('(')
		b.WriteString(Escape(v))
		b.WriteByte(')')
	}

	return b.String()
}

func (w WhereOR) String() string {
	return w.EscapeValue()
}

func (w WhereOR) EscapeValue() string {
	b := &strings.Builder{}

	for n, v := range w {
		if n > 0 {
			b.WriteString(" OR ")
		}
		b.WriteByte('(')
		b.WriteString(Escape(v))
		b.WriteByte(')')
	}

	return b.String()
}
