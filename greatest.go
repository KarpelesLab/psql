package psql

import "strings"

// Greatest creates a SQL GREATEST(a, b, ...) expression that returns the largest
// value among the arguments. On SQLite (which lacks GREATEST), it renders as a
// nested MAX() expression.
//
//	psql.Greatest(psql.F("user_count"), 0)
//	// MySQL/PG: → GREATEST("user_count",0)
//	// SQLite:   → MAX("user_count",0)
func Greatest(args ...any) EscapeValueable {
	return &greatestExpr{args: args, least: false}
}

// Least creates a SQL LEAST(a, b, ...) expression that returns the smallest
// value among the arguments. On SQLite (which lacks LEAST), it renders as a
// nested MIN() expression.
//
//	psql.Least(psql.F("stock"), 100)
//	// MySQL/PG: → LEAST("stock",100)
//	// SQLite:   → MIN("stock",100)
func Least(args ...any) EscapeValueable {
	return &greatestExpr{args: args, least: true}
}

type greatestExpr struct {
	args  []any
	least bool
}

func (g *greatestExpr) EscapeValue() string {
	return g.escapeValueCtx(nil)
}

func (g *greatestExpr) escapeValueCtx(ctx *renderContext) string {
	funcName := "GREATEST"
	if g.least {
		funcName = "LEAST"
	}
	// SQLite doesn't have GREATEST/LEAST but MAX/MIN work the same way
	if ctx != nil && ctx.e == EngineSQLite {
		if g.least {
			funcName = "MIN"
		} else {
			funcName = "MAX"
		}
	}

	b := &strings.Builder{}
	b.WriteString(funcName)
	b.WriteByte('(')
	for i, arg := range g.args {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(escapeCtx(ctx, arg))
	}
	b.WriteByte(')')
	return b.String()
}
