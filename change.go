package psql

import (
	"log"
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
		log.Printf("[psql] HasChanged but no state")
		return true
	}
	if !st.init {
		// uninitialized → no state
		log.Printf("[psql] HasChanged on non initialized value")
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
