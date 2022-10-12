package psql

import (
	"context"
	"reflect"
)

func FetchOne(ctx context.Context, target any, where map[string]any) error {
	table := GetTableMeta(reflect.TypeOf(target))
	return table.FetchOne(ctx, target, where)
}

func Fetch(ctx context.Context, obj any, where map[string]any) ([]any, error) {
	table := GetTableMeta(reflect.TypeOf(obj))
	return table.Fetch(ctx, where)
}
