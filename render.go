package psql

import (
	"strconv"
	"strings"
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
		b.WriteString(v.sortEscapeValue())
	}

	ctx.append(b.String())
	return nil
}

func (ctx *renderContext) appendArg(arg any) string {
	if ctx.useArgs {
		ctx.args = append(ctx.args, arg)
		switch ctx.e {
		case EnginePostgreSQL:
			return "$" + strconv.FormatUint(uint64(len(ctx.args)), 10)
		default:
			return "?"
		}
	}
	return Escape(arg)
}
