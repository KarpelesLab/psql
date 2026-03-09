package psql

import (
	"fmt"
	"log/slog"
	"reflect"
)

// HasChanged returns true if the object has been modified since it was last loaded
// from or saved to the database. It compares current field values against the stored
// state from the last scan.
func HasChanged[T any](obj *T) bool {
	return Table[T]().HasChanged(obj)
}

func (t *TableMeta[T]) HasChanged(obj *T) bool {
	st := t.rowstate(obj)
	if st == nil {
		// no main key → always report changed
		slog.Warn(fmt.Sprintf("[psql] HasChanged but no state"), "event", "psql:change:state_missing", "table", t.table)
		return true
	}
	if !st.init {
		// uninitialized → no state
		slog.Warn(fmt.Sprintf("[psql] HasChanged on non initialized value"), "event", "psql:change:state_uninit", "table", t.table)
		return true
	}

	val := reflect.ValueOf(obj).Elem()

	for _, col := range t.fields {
		// grab state value
		stv, ok := st.val[col.Column]
		if !ok {
			// can't check because that column wasn't fetched → bad
			return true
		}
		if !reflect.DeepEqual(val.Field(col.Index).Interface(), stv) {
			return true
		}
	}

	return false
}
