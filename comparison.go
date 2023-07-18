package psql

import "strings"

type Comparison struct {
	A, B any
	Op   string // one of "=", "<", ">", etc...
}

func Equal(a, b any) EscapeValueable {
	return &Comparison{a, b, "="}
}

func Gt(a, b any) EscapeValueable {
	return &Comparison{a, b, ">"}
}

func Gte(a, b any) EscapeValueable {
	return &Comparison{a, b, ">="}
}

func Lt(a, b any) EscapeValueable {
	return &Comparison{a, b, "<"}
}

func Lte(a, b any) EscapeValueable {
	return &Comparison{a, b, "<="}
}

func (c *Comparison) EscapeValue() string {
	// A Op B
	b := &strings.Builder{}
	b.WriteString(Escape(c.A))
	b.WriteString(c.Op)
	b.WriteString(Escape(c.B))
	return b.String()
}

func (c *Comparison) escapeValueCtx(ctx *renderContext) string {
	// A Op B
	b := &strings.Builder{}
	b.WriteString(escapeCtx(ctx, c.A))
	b.WriteString(c.Op)
	b.WriteString(escapeCtx(ctx, c.B))
	return b.String()
}

func (c *Comparison) sortEscapeValue() string {
	return c.EscapeValue()
}

func (c *Comparison) opStr(not bool) string {
	if !not {
		// as is
		return c.Op
	}
	// NOT
	switch c.Op {
	case "=":
		return "!="
	case "<":
		return ">="
	case "<=":
		return ">"
	case ">":
		return "<="
	case ">=":
		return "<"
	default:
		// ???
		// return "" so this leads to an error
		return ""
	}
}
