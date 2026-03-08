package psql

import "strings"

// Comparison represents a binary comparison expression (e.g., A = B, A > B).
// Use the helper constructors [Equal], [Gt], [Gte], [Lt], [Lte] instead of
// creating Comparison values directly.
type Comparison struct {
	A, B any
	Op   string // one of "=", "<", ">", etc...
}

// Equal creates an equality comparison (A = B). Use with [F] for field references:
//
//	psql.Equal(psql.F("status"), "active")
func Equal(a, b any) EscapeValueable {
	return &Comparison{a, b, "="}
}

// Gt creates a greater-than comparison (A > B).
func Gt(a, b any) EscapeValueable {
	return &Comparison{a, b, ">"}
}

// Gte creates a greater-than-or-equal comparison (A >= B).
func Gte(a, b any) EscapeValueable {
	return &Comparison{a, b, ">="}
}

// Lt creates a less-than comparison (A < B).
func Lt(a, b any) EscapeValueable {
	return &Comparison{a, b, "<"}
}

// Lte creates a less-than-or-equal comparison (A <= B).
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
