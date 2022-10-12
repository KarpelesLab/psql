package psql

import (
	"context"
	"reflect"
)

// Insert is a short way to insert objects into database
//
// psql.Insert(ctx, obj)
//
// Is equivalent to:
//
// psql.Table(obj).Insert(ctx, obj)
//
// All passed objects must be of the same type
func Insert(ctx context.Context, target ...interface{}) error {
	if len(target) == 0 {
		return nil
	}

	table := GetTableMeta(reflect.TypeOf(target[0]))
	return table.Insert(ctx, target...)
}
