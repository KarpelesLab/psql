package psql

import "strings"

type renderContext struct {
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
		return "?"
	}
	return Escape(arg)
}
