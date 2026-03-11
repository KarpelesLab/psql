package psql

import (
	"fmt"
	"strings"
	"time"
)

// Now returns a portable SQL expression for the current timestamp.
//
//	psql.Now()
//	// MySQL/PostgreSQL: NOW()
//	// SQLite:           CURRENT_TIMESTAMP
func Now() EscapeValueable {
	return &nowExpr{}
}

// DateAdd adds a Go [time.Duration] to a SQL timestamp expression, generating
// engine-appropriate interval arithmetic:
//
//	psql.DateAdd(psql.F("created_at"), 24*time.Hour)
//	// MySQL:      "created_at" + INTERVAL 1 DAY
//	// PostgreSQL: "created_at" + INTERVAL '1 day'
//	// SQLite:     datetime("created_at",'+1 day')
//
//	psql.DateAdd(psql.Now(), -30*time.Minute)
//	// MySQL:      NOW() - INTERVAL 30 MINUTE
//	// PostgreSQL: NOW() - INTERVAL '30 minute'
//	// SQLite:     datetime(CURRENT_TIMESTAMP,'-30 minutes')
func DateAdd(expr any, d time.Duration) EscapeValueable {
	return &dateAddExpr{expr: expr, d: d}
}

// DateSub subtracts a Go [time.Duration] from a SQL timestamp expression.
// It is equivalent to DateAdd(expr, -d).
func DateSub(expr any, d time.Duration) EscapeValueable {
	return &dateAddExpr{expr: expr, d: -d}
}

type nowExpr struct{}

func (n *nowExpr) EscapeValue() string {
	return n.escapeValueCtx(nil)
}

func (n *nowExpr) escapeValueCtx(ctx *renderContext) string {
	if ctx != nil && ctx.e == EngineSQLite {
		return "CURRENT_TIMESTAMP"
	}
	return "NOW()"
}

type dateAddExpr struct {
	expr any
	d    time.Duration
}

func (da *dateAddExpr) EscapeValue() string {
	return da.escapeValueCtx(nil)
}

func (da *dateAddExpr) escapeValueCtx(ctx *renderContext) string {
	d := da.d
	neg := d < 0
	if neg {
		d = -d
	}

	value, unit := durationParts(d)
	exprSQL := escapeCtx(ctx, da.expr)

	if ctx != nil && ctx.e == EngineSQLite {
		return da.renderSQLite(exprSQL, value, unit, neg)
	}

	// MySQL and PostgreSQL both support expr +/- INTERVAL syntax
	op := " + "
	if neg {
		op = " - "
	}

	if ctx != nil && ctx.e == EnginePostgreSQL {
		// PostgreSQL: expr + INTERVAL '5 second'
		return exprSQL + op + fmt.Sprintf("INTERVAL '%d %s'", value, unit)
	}

	// MySQL (and default): expr + INTERVAL 5 DAY
	return exprSQL + op + fmt.Sprintf("INTERVAL %d %s", value, strings.ToUpper(unit))
}

func (da *dateAddExpr) renderSQLite(exprSQL string, value int64, unit string, neg bool) string {
	sign := "+"
	if neg {
		sign = "-"
	}
	// SQLite: datetime(expr, '+5 seconds')
	// SQLite modifiers use plural forms for the modifier string
	return fmt.Sprintf("datetime(%s,'%s%d %ss')", exprSQL, sign, value, unit)
}

// durationParts decomposes a non-negative duration into (value, unit) using
// the largest whole unit that divides evenly.
func durationParts(d time.Duration) (int64, string) {
	us := d.Microseconds()

	if us == 0 {
		return 0, "second"
	}

	// Try from largest to smallest unit
	sec := us / 1_000_000
	if us%1_000_000 == 0 {
		if sec%86400 == 0 {
			return sec / 86400, "day"
		}
		if sec%3600 == 0 {
			return sec / 3600, "hour"
		}
		if sec%60 == 0 {
			return sec / 60, "minute"
		}
		return sec, "second"
	}

	// Sub-second: use microseconds
	return us, "microsecond"
}
