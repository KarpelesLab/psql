package psql

import "strings"

type Comparison struct {
	A, B any
	Op   string // one of "=", "<", ">", etc...
}

func Equal(a, b any) *Comparison {
	return &Comparison{a, b, "="}
}

func (c *Comparison) EscapeValue() string {
	// A Op B
	b := &strings.Builder{}
	b.WriteString(Escape(c.A))
	b.WriteString(c.Op)
	b.WriteString(Escape(c.B))
	return b.String()
}
