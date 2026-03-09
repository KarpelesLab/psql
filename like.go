package psql

import (
	"strings"
)

// Like represents a SQL LIKE condition. Use in WHERE clauses:
//
//	psql.B().Select().From("users").Where(&psql.Like{Field: psql.F("name"), Like: "John%"})
//
// Set CaseInsensitive to true for case-insensitive matching. This renders as
// ILIKE on PostgreSQL, LIKE on MySQL (case-insensitive by default collation),
// and LIKE with COLLATE NOCASE on SQLite.
type Like struct {
	Field           any
	Like            string
	CaseInsensitive bool
}

func (l *Like) String() string {
	return l.EscapeValue()
}

func (l *Like) EscapeValue() string {
	return l.escapeValueCtx(nil)
}

func (l *Like) escapeValueCtx(ctx *renderContext) string {
	b := &strings.Builder{}
	b.WriteString(escapeCtx(ctx, l.Field))

	keyword := "LIKE"
	suffix := " ESCAPE '\\'"

	if l.CaseInsensitive && ctx != nil {
		switch ctx.e {
		case EnginePostgreSQL:
			keyword = "ILIKE"
		case EngineSQLite:
			suffix = " ESCAPE '\\' COLLATE NOCASE"
		}
	}

	b.WriteByte(' ')
	b.WriteString(keyword)
	b.WriteByte(' ')
	b.WriteString(escapeCtx(ctx, l.Like))
	b.WriteString(suffix)

	return b.String()
}
