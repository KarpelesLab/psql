package psql

import (
	"strings"
)

// FindInSet represents a MySQL FIND_IN_SET() function call, which searches for
// a string value within a comma-separated list stored in a column.
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
