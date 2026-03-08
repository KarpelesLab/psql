package psql_test

import (
	"context"
	"sync"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type LazyTable struct {
	psql.Name `sql:"test_lazy"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestLazy(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_lazy\"").Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &LazyTable{ID: 1, Label: "one"}))
	require.NoError(t, psql.Insert(ctx, &LazyTable{ID: 2, Label: "two"}))
	require.NoError(t, psql.Insert(ctx, &LazyTable{ID: 3, Label: "three"}))

	t.Run("SingleResolve", func(t *testing.T) {
		f := psql.Lazy[LazyTable]("ID", "1")
		obj, err := f.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "one", obj.Label)
	})

	t.Run("BatchResolve", func(t *testing.T) {
		// Create multiple futures for the same column before resolving any
		f1 := psql.Lazy[LazyTable]("ID", "1")
		f2 := psql.Lazy[LazyTable]("ID", "2")
		f3 := psql.Lazy[LazyTable]("ID", "3")

		// Resolve the first — should batch-resolve all three
		obj1, err := f1.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "one", obj1.Label)

		// These should already be resolved by the batch
		obj2, err := f2.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "two", obj2.Label)

		obj3, err := f3.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "three", obj3.Label)
	})

	t.Run("ConcurrentResolve", func(t *testing.T) {
		// Create futures
		f1 := psql.Lazy[LazyTable]("ID", "1")
		f2 := psql.Lazy[LazyTable]("ID", "2")
		f3 := psql.Lazy[LazyTable]("ID", "3")

		// Resolve concurrently
		var wg sync.WaitGroup
		results := make([]*LazyTable, 3)
		errs := make([]error, 3)

		wg.Add(3)
		go func() { defer wg.Done(); results[0], errs[0] = f1.Resolve(ctx) }()
		go func() { defer wg.Done(); results[1], errs[1] = f2.Resolve(ctx) }()
		go func() { defer wg.Done(); results[2], errs[2] = f3.Resolve(ctx) }()
		wg.Wait()

		for i, err := range errs {
			require.NoError(t, err, "future %d should resolve without error", i+1)
		}
		assert.Equal(t, "one", results[0].Label)
		assert.Equal(t, "two", results[1].Label)
		assert.Equal(t, "three", results[2].Label)
	})

	t.Run("NotFound", func(t *testing.T) {
		// Create a future for a non-existent ID alongside an existing one
		f1 := psql.Lazy[LazyTable]("ID", "1")
		f2 := psql.Lazy[LazyTable]("ID", "999")

		// Resolve the existing one — should batch both
		obj1, err := f1.Resolve(ctx)
		require.NoError(t, err)
		assert.Equal(t, "one", obj1.Label)

		// The non-existent one should return an error
		_, err = f2.Resolve(ctx)
		assert.Error(t, err, "non-existent record should return error")
	})

	_ = psql.Q("DROP TABLE IF EXISTS \"test_lazy\"").Exec(ctx)
}
