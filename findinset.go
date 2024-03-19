package psql

import (
	"strings"
)

type FindInSet struct {
	Field any
	Value string
}

func (f *FindInSet) String() string {
	return f.EscapeValue()
}

func (f *FindInSet) EscapeValue() string {
	b := &strings.Builder{}
	b.WriteString("FIND_IN_SET(")
	b.WriteString(Escape(f.Value))
	b.WriteString(",")
	b.WriteString(Escape(f.Field))
	b.WriteString(")")

	return b.String()
}

func (f *FindInSet) escapeValueCtx(ctx *renderContext) string {
	return f.EscapeValue()
}
