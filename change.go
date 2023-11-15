package psql

import (
	"fmt"
	"log/slog"
	"reflect"
)

func HasChanged[T any](obj *T) bool {
	// report if object has been updated
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
		stv, ok := st.val[col.column]
		if !ok {
			// can't check because that column wasn't fetched → bad
			//log.Printf("[psql] HasChanged can't check value for %s", col.column)
			return true
		}
		if !reflect.DeepEqual(val.Field(col.index).Interface(), stv) {
			//log.Printf("[psql] found diff in field %s: %v != %v", col.column, val.Field(col.index).Interface(), stv)
			return true
		}
	}

	return false
}
