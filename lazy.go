package psql

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/KarpelesLab/pjson"
)

// Future represents a lazily-loaded database record. Created by [Lazy], it defers
// the actual database query until [Future.Resolve] is called. When resolved, it
// automatically batches all pending futures for the same table and column into a
// single query, significantly reducing database round trips.
//
// Concurrent Resolve calls share the same result. Future also implements json.Marshaler.
type Future[T any] struct {
	col   string
	val   string
	obj   *T
	err   error
	k     string
	done  uint32 // atomic: 0=pending, 1=resolved
	wait  chan struct{}
	table *TableMeta[T]
}

// Lazy returns an instance of Future that will be resolved in the future. Multiple calls
// to Lazy in different goroutines will return the same value until it is resolved.
//
// When any Future is resolved, all pending futures for the same table and column are
// batched into a single WHERE col IN (...) query, reducing database round trips.
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
		wait:  make(chan struct{}),
		table: t,
	}

	v, ok = t.futures.LoadOrStore(k, newv)
	if ok {
		return v.(*Future[T])
	}

	return newv
}

func (f *Future[T]) Resolve(ctx context.Context) (*T, error) {
	if atomic.LoadUint32(&f.done) == 1 {
		return f.obj, f.err
	}

	if ctx == nil {
		ctx = context.Background()
	}

	// Try to be the resolver for this future
	f.resolve(ctx)

	// Wait until resolved (by us or by a batch leader)
	<-f.wait
	return f.obj, f.err
}

// resolveMu serializes batch resolution for a given table to prevent
// concurrent leaders from seeing the same peers.
var resolveMu sync.Mutex

func (f *Future[T]) resolve(ctx context.Context) {
	// Quick check: already resolved by a batch leader
	if atomic.LoadUint32(&f.done) == 1 {
		return
	}

	resolveMu.Lock()
	// Double-check after acquiring lock
	if atomic.LoadUint32(&f.done) == 1 {
		resolveMu.Unlock()
		return
	}

	// Check if another leader already claimed us as a peer
	if _, stillInMap := f.table.futures.Load(f.k); !stillInMap {
		resolveMu.Unlock()
		return
	}

	// Collect all pending futures for the same column
	prefix := f.col + "\x00"
	var peers []*Future[T]

	f.table.futures.Range(func(key, value any) bool {
		k := key.(string)
		peer := value.(*Future[T])
		if peer != f && strings.HasPrefix(k, prefix) && atomic.LoadUint32(&peer.done) == 0 {
			peers = append(peers, peer)
		}
		return true
	})

	// Remove self and peers from the futures map so no other leader claims them
	f.table.futures.Delete(f.k)
	for _, p := range peers {
		f.table.futures.Delete(p.k)
	}

	resolveMu.Unlock()

	// Now resolve outside the lock
	if len(peers) == 0 {
		// Single fetch
		f.obj, f.err = f.table.Get(ctx, map[string]any{f.col: f.val})
		atomic.StoreUint32(&f.done, 1)
		close(f.wait)
		return
	}

	// Batch fetch
	vals := make([]any, 0, len(peers)+1)
	vals = append(vals, f.val)
	for _, p := range peers {
		vals = append(vals, p.val)
	}

	results, err := f.table.Fetch(ctx, map[string]any{f.col: vals})

	// Build result index by column value (as string)
	resultByVal := make(map[string]*T)
	if err == nil {
		fld := f.table.fldcol[f.col]
		if fld != nil {
			for _, r := range results {
				key := fmt.Sprintf("%v", reflect.ValueOf(r).Elem().Field(fld.index).Interface())
				resultByVal[key] = r
			}
		}
	}

	// Set our own result
	if err != nil {
		f.err = err
	} else if obj, ok := resultByVal[f.val]; ok {
		f.obj = obj
	} else {
		f.err = os.ErrNotExist
	}
	atomic.StoreUint32(&f.done, 1)
	close(f.wait)

	// Distribute results to peers
	for _, p := range peers {
		if err != nil {
			p.err = err
		} else if obj, ok := resultByVal[p.val]; ok {
			p.obj = obj
		} else {
			p.err = os.ErrNotExist
		}
		atomic.StoreUint32(&p.done, 1)
		close(p.wait)
	}
}

func (f *Future[T]) MarshalJSON() ([]byte, error) {
	v, err := f.Resolve(nil)
	if err != nil {
		return nil, err
	}
	return pjson.Marshal(v)
}

func (f *Future[T]) MarshalContextJSON(ctx context.Context) ([]byte, error) {
	v, err := f.Resolve(ctx)
	if err != nil {
		return nil, err
	}
	return pjson.Marshal(v)
}
