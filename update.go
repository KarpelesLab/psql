package psql

import (
	"context"
	"errors"
	"log"
	"reflect"
)

// Update is a short way to insert objects into database
//
// psql.Update(ctx, obj)
//
// Is equivalent to:
//
// psql.Table(obj).Update(ctx, obj)
//
// All passed objects must be of the same type
func Update[T any](ctx context.Context, target ...*T) error {
	if len(target) == 0 {
		return nil
	}

	// (TODO: need key)
	return errors.New("TODO")
	//table := GetTableMeta(reflect.TypeOf(target[0]))
	//return table.Update(ctx, target...)
}

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
