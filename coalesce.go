package psql

import "strings"

// Coalesce creates a SQL COALESCE(a, b, ...) expression that returns the first
// non-NULL argument. Arguments can be field references, values, or subqueries:
//
//	psql.Coalesce(psql.F("nickname"), psql.F("name"))
//	// → COALESCE("nickname","name")
//
//	psql.Coalesce(
//	    psql.B().Select(psql.Raw("COUNT(*)")).From("messages").Where(...),
//	    0,
//	)
//	// → COALESCE((SELECT COUNT(*) FROM "messages" WHERE ...), 0)
func Coalesce(args ...any) EscapeValueable {
	return &coalesceExpr{args: args}
}

type coalesceExpr struct {
	args []any
}

func (c *coalesceExpr) EscapeValue() string {
	return c.escapeValueCtx(nil)
}

func (c *coalesceExpr) escapeValueCtx(ctx *renderContext) string {
	b := &strings.Builder{}
	b.WriteString("COALESCE(")
	for i, arg := range c.args {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(escapeCtx(ctx, arg))
	}
	b.WriteByte(')')
	return b.String()
}
