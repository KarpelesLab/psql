package psql

import (
	"context"
	"errors"
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
	return Table(obj).HasChanged(obj)
}

func (t *TableMeta[T]) HasChanged(obj any) bool {
	if t.mainKey == nil {
		// no main key → always report changed
		return true
	}

	// TODO
	return true
}
