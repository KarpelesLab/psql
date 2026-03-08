package psql

import (
	"strconv"
	"strings"
	"time"
)

type renderContext struct {
	e       Engine
	req     []string
	args    []any
	useArgs bool
}

func (ctx *renderContext) append(v ...string) {
	ctx.req = append(ctx.req, v...)
}

func (ctx *renderContext) appendCommaValues(vals ...any) error {
	b := &strings.Builder{}

	for n, v := range vals {
		if n != 0 {
			b.WriteByte(',')
		}
		b.WriteString(escapeCtx(ctx, v))
	}

	ctx.append(b.String())
	return nil
}

func (ctx *renderContext) appendCommaValuesSort(vals ...SortValueable) error {
	b := &strings.Builder{}

	for n, v := range vals {
		if n != 0 {
			b.WriteByte(',')
		}
		// Use engine-aware rendering if available (e.g., vector distance operators)
		if vc, ok := v.(sortValueCtxable); ok {
			b.WriteString(vc.sortEscapeValueCtx(ctx))
		} else {
			b.WriteString(v.sortEscapeValue())
		}
	}

	ctx.append(b.String())
	return nil
}

// sortValueCtxable is an optional interface for SortValueable implementations
// that need engine-aware rendering (e.g., PostgreSQL vector operators).
type sortValueCtxable interface {
	sortEscapeValueCtx(ctx *renderContext) string
}

func (ctx *renderContext) appendArg(arg any) string {
	if ctx.useArgs {
		// SQLite driver doesn't handle time.Time natively; format as RFC3339Nano string
		if ctx.e == EngineSQLite {
			switch v := arg.(type) {
			case time.Time:
				arg = v.UTC().Format(time.RFC3339Nano)
			case *time.Time:
				if v != nil {
					arg = v.UTC().Format(time.RFC3339Nano)
				}
			}
		}
		ctx.args = append(ctx.args, arg)
		switch ctx.e {
		case EnginePostgreSQL:
			return "$" + strconv.FormatUint(uint64(len(ctx.args)), 10)
		default: // MySQL, SQLite both use ?
			return "?"
		}
	}
	return Escape(arg)
}
