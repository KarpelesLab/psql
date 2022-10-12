package psql

import (
	"strings"
)

type Like struct {
	Field any
	Like  string
}

func (l *Like) String() string {
	return l.EscapeValue()
}

func (l *Like) EscapeValue() string {
	// We enforce NO_BACKSLASH_ESCAPES
	b := &strings.Builder{}
	b.WriteString(Escape(l.Field))
	b.WriteString(" LIKE ")
	b.WriteString(Escape(l.Like))
	b.WriteString(" ESCAPE '\\'")

	return b.String()
}
