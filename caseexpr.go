package psql

import "strings"

// caseExpr represents a SQL CASE expression. Use [Case] to create one and chain
// [caseExpr.When] and [caseExpr.Else] to build it:
//
//	// Searched CASE (no operand):
//	psql.Case().
//	    When(psql.Gt(psql.F("age"), 18), "adult").
//	    When(psql.Gt(psql.F("age"), 12), "teen").
//	    Else("child")
//	// → CASE WHEN "age">18 THEN 'adult' WHEN "age">12 THEN 'teen' ELSE 'child' END
//
//	// Simple CASE (with operand):
//	psql.Case(psql.F("status")).
//	    When("active", 1).
//	    When("inactive", 0).
//	    Else(-1)
//	// → CASE "status" WHEN 'active' THEN 1 WHEN 'inactive' THEN 0 ELSE -1 END
type caseExpr struct {
	operand any // nil for searched CASE
	whens   []whenClause
	elseVal any
	hasElse bool
}

type whenClause struct {
	when any
	then any
}

// Case creates a new CASE expression. Pass no arguments for a searched CASE
// (CASE WHEN condition THEN ...), or pass one argument for a simple CASE
// (CASE expr WHEN value THEN ...).
func Case(operand ...any) *caseExpr {
	c := &caseExpr{}
	if len(operand) == 1 {
		c.operand = operand[0]
	}
	return c
}

// When adds a WHEN ... THEN ... clause to the CASE expression.
func (c *caseExpr) When(when, then any) *caseExpr {
	c.whens = append(c.whens, whenClause{when: when, then: then})
	return c
}

// Else sets the ELSE value for the CASE expression.
func (c *caseExpr) Else(val any) *caseExpr {
	c.elseVal = val
	c.hasElse = true
	return c
}

func (c *caseExpr) EscapeValue() string {
	return c.escapeValueCtx(nil)
}

func (c *caseExpr) escapeValueCtx(ctx *renderContext) string {
	b := &strings.Builder{}
	b.WriteString("CASE")
	if c.operand != nil {
		b.WriteByte(' ')
		b.WriteString(escapeCtx(ctx, c.operand))
	}
	for _, w := range c.whens {
		b.WriteString(" WHEN ")
		b.WriteString(escapeCtx(ctx, w.when))
		b.WriteString(" THEN ")
		b.WriteString(escapeCtx(ctx, w.then))
	}
	if c.hasElse {
		b.WriteString(" ELSE ")
		b.WriteString(escapeCtx(ctx, c.elseVal))
	}
	b.WriteString(" END")
	return b.String()
}
