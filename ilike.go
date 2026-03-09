package psql

import (
	"strings"
)

// ILike represents a case-insensitive SQL LIKE condition. On PostgreSQL it renders
// as ILIKE, on MySQL it renders as LIKE (case-insensitive by default collation),
// and on SQLite it uses LIKE with COLLATE NOCASE.
type ILike struct {
	Field any
	Like  string
}

func (l *ILike) String() string {
	return l.EscapeValue()
}

func (l *ILike) EscapeValue() string {
	return l.escapeValueCtx(nil)
}

func (l *ILike) escapeValueCtx(ctx *renderContext) string {
	b := &strings.Builder{}
	b.WriteString(escapeCtx(ctx, l.Field))

	keyword := "LIKE"
	suffix := " ESCAPE '\\'"
	if ctx != nil {
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
