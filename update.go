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
func Update(ctx context.Context, target ...interface{}) error {
	if len(target) == 0 {
		return nil
	}

	// (TODO: need key)
	return errors.New("TODO")
	//table := GetTableMeta(reflect.TypeOf(target[0]))
	//return table.Update(ctx, target...)
}
