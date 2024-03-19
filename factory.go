package psql

import (
	"reflect"

	"github.com/KarpelesLab/typutil"
)

// Factory returns a new object T pre-initialized with its defaults
func (t *TableMeta[T]) Factory() *T {
	objptr := reflect.New(t.typ)
	obj := objptr.Elem()

	for _, f := range t.fields {
		def, ok := f.attrs["default"]
		if !ok {
			continue
		}
		// handle errors?
		typutil.AssignReflect(obj.Field(f.index), reflect.ValueOf(def))
	}
	return objptr.Interface().(*T)
}
