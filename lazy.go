package psql

import (
	"context"
	"sync"

	"github.com/KarpelesLab/pjson"
)

type Future[T any] struct {
	col   string
	val   string
	obj   *T
	err   error
	k     string
	once  sync.Once
	table *TableMeta[T]
}

// Lazy returns an instance of Future that will be resolved in the future. Multiple calls
// to Lazy in different goroutines will return the same value until it is resolved. This
// will also attempt to group requests to the same table in the future.
func Lazy[T any](col, val string) *Future[T] {
	t := Table[T]()
	k := col + "\x00" + val

	v, ok := t.futures.Load(k)
	if ok {
		return v.(*Future[T])
	}

	newv := &Future[T]{
		col:   col,
		val:   val,
		k:     k,
		table: t,
	}

	v, ok = t.futures.LoadOrStore(k, newv)
	if ok {
		return v.(*Future[T])
	}

	return newv
}

func (f *Future[T]) resolve(ctx context.Context) (*T, error) {
	f.once.Do(func() {
		if ctx == nil {
			ctx = context.Background()
		}

		f.obj, f.err = f.table.Get(ctx, map[string]any{f.col: f.val})

		// remove from cache but only if we're still in there
		f.table.futures.CompareAndDelete(f.k, f)
	})

	return f.obj, f.err
}

func (f *Future[T]) MarshalJSON() ([]byte, error) {
	v, err := f.resolve(nil)
	if err != nil {
		return nil, err
	}
	return pjson.Marshal(v)
}

func (f *Future[T]) MarshalContextJSON(ctx context.Context) ([]byte, error) {
	v, err := f.resolve(ctx)
	if err != nil {
		return nil, err
	}
	return pjson.Marshal(v)
}
