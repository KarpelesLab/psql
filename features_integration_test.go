package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === IsDuplicate integration tests ===

type DupTable struct {
	psql.Name `sql:"test_dup"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestIsDuplicateIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_dup"`).Exec(ctx)

	err := psql.Insert(ctx, &DupTable{ID: 1, Label: "first"})
	require.NoError(t, err)

	// Insert duplicate key
	err = psql.Insert(ctx, &DupTable{ID: 1, Label: "second"})
	require.Error(t, err)
	assert.True(t, psql.IsDuplicate(err), "expected IsDuplicate to be true, got error: %v", err)

	// Non-duplicate error should return false
	assert.False(t, psql.IsDuplicate(nil))

	_ = psql.Q(`DROP TABLE IF EXISTS "test_dup"`).Exec(ctx)
}

// === ILike integration tests ===

type ILikeTable struct {
	psql.Name `sql:"test_ilike"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestILikeIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_ilike"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &ILikeTable{ID: 1, Label: "Hello World"}))
	require.NoError(t, psql.Insert(ctx, &ILikeTable{ID: 2, Label: "HELLO THERE"}))
	require.NoError(t, psql.Insert(ctx, &ILikeTable{ID: 3, Label: "goodbye"}))

	// ILike should match case-insensitively
	results, err := psql.Fetch[ILikeTable](ctx, map[string]any{
		"Label": psql.ILike{Like: "hello%"},
	})
	require.NoError(t, err)
	assert.Len(t, results, 2, "ILike should match both 'Hello World' and 'HELLO THERE'")

	// Regular Like for comparison (may be case-sensitive on PG)
	results, err = psql.Fetch[ILikeTable](ctx, map[string]any{
		"Label": psql.Like{Like: "hello%"},
	})
	require.NoError(t, err)
	// Result count depends on engine (PG: 0, MySQL: 2, SQLite: 2)
	// Just verify no error

	// NOT ILike
	results, err = psql.Fetch[ILikeTable](ctx, map[string]any{
		"Label": &psql.Not{V: psql.ILike{Like: "hello%"}},
	})
	require.NoError(t, err)
	assert.Len(t, results, 1, "NOT ILike should return only 'goodbye'")
	if len(results) > 0 {
		assert.Equal(t, "goodbye", results[0].Label)
	}

	_ = psql.Q(`DROP TABLE IF EXISTS "test_ilike"`).Exec(ctx)
}

// === Typed slices IN() integration tests ===

type TypedINTable struct {
	psql.Name `sql:"test_typed_in"`
	ID        int64 `sql:",key=PRIMARY"`
	Score     int64
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestTypedSliceINIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_typed_in"`).Exec(ctx)

	for i := int64(1); i <= 5; i++ {
		require.NoError(t, psql.Insert(ctx, &TypedINTable{ID: i, Score: i * 10, Label: "item"}))
	}

	t.Run("[]int64", func(t *testing.T) {
		results, err := psql.Fetch[TypedINTable](ctx, map[string]any{
			"ID": []int64{1, 3, 5},
		})
		require.NoError(t, err)
		assert.Len(t, results, 3)
	})

	t.Run("[]int", func(t *testing.T) {
		results, err := psql.Fetch[TypedINTable](ctx, map[string]any{
			"ID": []int{2, 4},
		})
		require.NoError(t, err)
		assert.Len(t, results, 2)
	})

	t.Run("NOT []int64", func(t *testing.T) {
		results, err := psql.Fetch[TypedINTable](ctx, map[string]any{
			"ID": &psql.Not{V: []int64{1, 2}},
		})
		require.NoError(t, err)
		assert.Len(t, results, 3) // IDs 3,4,5
	})

	t.Run("empty slice", func(t *testing.T) {
		results, err := psql.Fetch[TypedINTable](ctx, map[string]any{
			"ID": []int64{},
		})
		require.NoError(t, err)
		assert.Len(t, results, 0)
	})

	_ = psql.Q(`DROP TABLE IF EXISTS "test_typed_in"`).Exec(ctx)
}

// === Incr/Decr integration tests ===

type CounterTable struct {
	psql.Name `sql:"test_counter"`
	ID        int64 `sql:",key=PRIMARY"`
	Views     int64
	Stock     int64
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestIncrDecrIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_counter"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &CounterTable{ID: 1, Views: 100, Stock: 50, Label: "item"}))

	t.Run("Increment", func(t *testing.T) {
		_, err := psql.B().Update("test_counter").
			Set(map[string]any{"Views": psql.Incr(int64(5))}).
			Where(map[string]any{"ID": int64(1)}).
			ExecQuery(ctx)
		require.NoError(t, err)

		obj, err := psql.Get[CounterTable](ctx, map[string]any{"ID": int64(1)})
		require.NoError(t, err)
		assert.Equal(t, int64(105), obj.Views)
	})

	t.Run("Decrement", func(t *testing.T) {
		_, err := psql.B().Update("test_counter").
			Set(map[string]any{"Stock": psql.Decr(int64(3))}).
			Where(map[string]any{"ID": int64(1)}).
			ExecQuery(ctx)
		require.NoError(t, err)

		obj, err := psql.Get[CounterTable](ctx, map[string]any{"ID": int64(1)})
		require.NoError(t, err)
		assert.Equal(t, int64(47), obj.Stock)
	})

	_ = psql.Q(`DROP TABLE IF EXISTS "test_counter"`).Exec(ctx)
}

// === FOR UPDATE SKIP LOCKED integration tests ===

type LockTable struct {
	psql.Name `sql:"test_lock"`
	ID        int64  `sql:",key=PRIMARY"`
	Status    string `sql:",type=VARCHAR,size=64"`
}

func TestForUpdateSkipLockedIntegration(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() == psql.EngineSQLite {
		t.Skip("SQLite does not support FOR UPDATE")
	}
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_lock"`).Exec(ctx)

	for i := int64(1); i <= 3; i++ {
		require.NoError(t, psql.Insert(ctx, &LockTable{ID: i, Status: "pending"}))
	}

	// Test that SKIP LOCKED query at least doesn't error
	results, err := psql.Fetch[LockTable](ctx,
		map[string]any{"Status": "pending"},
		psql.FetchLockSkipLocked,
	)
	require.NoError(t, err)
	assert.Len(t, results, 3)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_lock"`).Exec(ctx)
}

func TestForUpdateNoWaitIntegration(t *testing.T) {
	be := getTestBackend(t)
	if be.Engine() == psql.EngineSQLite {
		t.Skip("SQLite does not support FOR UPDATE")
	}
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_lock"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &LockTable{ID: 1, Status: "pending"}))

	// Test that NOWAIT query at least doesn't error when no locks
	results, err := psql.Fetch[LockTable](ctx,
		map[string]any{"Status": "pending"},
		psql.FetchLockNoWait,
	)
	require.NoError(t, err)
	assert.Len(t, results, 1)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_lock"`).Exec(ctx)
}

// === RETURNING integration tests ===

type RetTable struct {
	psql.Name `sql:"test_returning"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
	Score     int64
}

func TestReturningInsert(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_returning"`).Exec(ctx)

	obj := &RetTable{ID: 1, Label: "inserted", Score: 42}
	err := psql.Insert(ctx, obj)
	require.NoError(t, err)

	// On PG, RETURNING should have scanned values back
	// On MySQL/SQLite, the struct should still have its original values
	assert.Equal(t, int64(1), obj.ID)
	assert.Equal(t, "inserted", obj.Label)
	assert.Equal(t, int64(42), obj.Score)

	// Verify in database
	fetched, err := psql.Get[RetTable](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Equal(t, "inserted", fetched.Label)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_returning"`).Exec(ctx)
}

func TestReturningReplace(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_returning"`).Exec(ctx)

	// Insert first
	err := psql.Insert(ctx, &RetTable{ID: 1, Label: "original", Score: 10})
	require.NoError(t, err)

	// Replace
	obj := &RetTable{ID: 1, Label: "replaced", Score: 99}
	err = psql.Replace(ctx, obj)
	require.NoError(t, err)

	// On PG with RETURNING, obj should have updated values
	assert.Equal(t, int64(1), obj.ID)
	assert.Equal(t, "replaced", obj.Label)
	assert.Equal(t, int64(99), obj.Score)

	// Verify in database
	fetched, err := psql.Get[RetTable](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Equal(t, "replaced", fetched.Label)
	assert.Equal(t, int64(99), fetched.Score)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_returning"`).Exec(ctx)
}

func TestReturningInsertIgnore(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_returning"`).Exec(ctx)

	err := psql.Insert(ctx, &RetTable{ID: 1, Label: "first", Score: 10})
	require.NoError(t, err)

	// InsertIgnore with conflicting key
	obj := &RetTable{ID: 1, Label: "second", Score: 20}
	err = psql.InsertIgnore(ctx, obj)
	require.NoError(t, err)

	// DB should still have original values
	fetched, err := psql.Get[RetTable](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Equal(t, "first", fetched.Label)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_returning"`).Exec(ctx)
}

// === Subquery in WHERE integration tests ===

type SubqParent struct {
	psql.Name `sql:"test_subq_parent"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

type SubqChild struct {
	psql.Name `sql:"test_subq_child"`
	ID        int64 `sql:",key=PRIMARY"`
	ParentID  int64
	Score     int64
}

func TestSubqueryIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_subq_child"`).Exec(ctx)
	_ = psql.Q(`DROP TABLE IF EXISTS "test_subq_parent"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &SubqParent{ID: 1, Label: "parent1"}))
	require.NoError(t, psql.Insert(ctx, &SubqParent{ID: 2, Label: "parent2"}))
	require.NoError(t, psql.Insert(ctx, &SubqParent{ID: 3, Label: "parent3"}))

	require.NoError(t, psql.Insert(ctx, &SubqChild{ID: 1, ParentID: 1, Score: 100}))
	require.NoError(t, psql.Insert(ctx, &SubqChild{ID: 2, ParentID: 2, Score: 200}))

	t.Run("IN subquery", func(t *testing.T) {
		sub := psql.B().Select("ParentID").From("test_subq_child")
		results, err := psql.Fetch[SubqParent](ctx, map[string]any{
			"ID": &psql.SubIn{Sub: sub},
		})
		require.NoError(t, err)
		assert.Len(t, results, 2, "should find parents with children")
	})

	t.Run("NOT IN subquery", func(t *testing.T) {
		sub := psql.B().Select("ParentID").From("test_subq_child")
		results, err := psql.Fetch[SubqParent](ctx, map[string]any{
			"ID": &psql.Not{V: &psql.SubIn{Sub: sub}},
		})
		require.NoError(t, err)
		assert.Len(t, results, 1, "should find parent without children")
		if len(results) > 0 {
			assert.Equal(t, "parent3", results[0].Label)
		}
	})

	t.Run("scalar subquery", func(t *testing.T) {
		// Find the parent whose ID equals the max ParentID in children
		sub := psql.B().Select(psql.Raw("MAX(\"ParentID\")")).From("test_subq_child")
		results, err := psql.Fetch[SubqParent](ctx, map[string]any{
			"ID": sub,
		})
		require.NoError(t, err)
		assert.Len(t, results, 1)
		if len(results) > 0 {
			assert.Equal(t, "parent2", results[0].Label)
		}
	})

	_ = psql.Q(`DROP TABLE IF EXISTS "test_subq_child"`).Exec(ctx)
	_ = psql.Q(`DROP TABLE IF EXISTS "test_subq_parent"`).Exec(ctx)
}

// === Raw in WHERE integration tests ===

type RawWhereTable struct {
	psql.Name `sql:"test_raw_where"`
	ID        int64 `sql:",key=PRIMARY"`
	Score     int64
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestRawWhereIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_raw_where"`).Exec(ctx)

	for i := int64(1); i <= 5; i++ {
		require.NoError(t, psql.Insert(ctx, &RawWhereTable{ID: i, Score: i * 10, Label: "item"}))
	}

	// Use Raw in WHERE
	results, err := psql.Fetch[RawWhereTable](ctx, psql.Raw(`"Score" > 30`))
	require.NoError(t, err)
	assert.Len(t, results, 2, "should find items with score > 30")

	_ = psql.Q(`DROP TABLE IF EXISTS "test_raw_where"`).Exec(ctx)
}

// === RunQueryT integration tests ===

type QueryTTable struct {
	psql.Name `sql:"test_query_t"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
	Score     int64
}

func TestRunQueryTIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_query_t"`).Exec(ctx)

	for i := int64(1); i <= 3; i++ {
		require.NoError(t, psql.Insert(ctx, &QueryTTable{ID: i, Label: "item", Score: i * 10}))
	}

	t.Run("RunQueryT", func(t *testing.T) {
		q := psql.B().Select(psql.Raw(`"ID","Label","Score"`)).From("test_query_t").OrderBy(psql.S("ID", "ASC"))
		results, err := psql.RunQueryT[QueryTTable](ctx, q)
		require.NoError(t, err)
		assert.Len(t, results, 3)
		if len(results) >= 3 {
			assert.Equal(t, int64(1), results[0].ID)
			assert.Equal(t, int64(30), results[2].Score)
		}
	})

	t.Run("RunQueryTOne", func(t *testing.T) {
		q := psql.B().Select(psql.Raw(`"ID","Label","Score"`)).From("test_query_t").Where(map[string]any{"ID": int64(2)})
		result, err := psql.RunQueryTOne[QueryTTable](ctx, q)
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.ID)
		assert.Equal(t, int64(20), result.Score)
	})

	t.Run("RunQueryTOne not found", func(t *testing.T) {
		q := psql.B().Select(psql.Raw(`"ID","Label","Score"`)).From("test_query_t").Where(map[string]any{"ID": int64(999)})
		_, err := psql.RunQueryTOne[QueryTTable](ctx, q)
		assert.Error(t, err) // os.ErrNotExist
	})

	_ = psql.Q(`DROP TABLE IF EXISTS "test_query_t"`).Exec(ctx)
}

// === SetRaw integration tests ===

func TestSetRawIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_counter"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &CounterTable{ID: 1, Views: 100, Stock: 50, Label: "item"}))

	// SetRaw with a literal value
	_, err := psql.B().Update("test_counter").
		Set(map[string]any{"Views": &psql.SetRaw{SQL: "0"}}).
		Where(map[string]any{"ID": int64(1)}).
		ExecQuery(ctx)
	require.NoError(t, err)

	obj, err := psql.Get[CounterTable](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Equal(t, int64(0), obj.Views)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_counter"`).Exec(ctx)
}
