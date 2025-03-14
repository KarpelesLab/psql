package psql

import (
	"context"
	"reflect"

	"github.com/KarpelesLab/typutil"
)

// Factory returns a new object T pre-initialized with its defaults
func Factory[T any](ctxs ...context.Context) *T {
	if len(ctxs) == 0 {
		ctxs = []context.Context{context.Background()}
	}
	return Table[T]().Factory(ctxs[0])
}

// Factory returns a new object T pre-initialized with its defaults
func (t *TableMeta[T]) Factory(ctx context.Context) *T {
	objptr := reflect.New(t.typ)
	obj := objptr.Elem()

	for _, f := range t.fields {
		def, ok := f.getAttrs(GetBackend(ctx))["default"]
		if !ok {
			continue
		}
		// handle errors?
		typutil.AssignReflect(obj.Field(f.index), reflect.ValueOf(def))
	}
	return objptr.Interface().(*T)
}
