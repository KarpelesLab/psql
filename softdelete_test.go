package psql_test

import (
	"context"
	"testing"
	"time"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type SoftDelTable struct {
	psql.Name `sql:"test_softdel"`
	ID        int64      `sql:",key=PRIMARY"`
	Label     string     `sql:",type=VARCHAR,size=128"`
	DeletedAt *time.Time `sql:",type=TIMESTAMPTZ"`
}

func TestSoftDelete(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_softdel\"").Exec(ctx)

	// Insert test data
	require.NoError(t, psql.Insert(ctx, &SoftDelTable{ID: 1, Label: "keep"}))
	require.NoError(t, psql.Insert(ctx, &SoftDelTable{ID: 2, Label: "delete_me"}))
	require.NoError(t, psql.Insert(ctx, &SoftDelTable{ID: 3, Label: "also_keep"}))

	// Soft delete one record
	res, err := psql.Delete[SoftDelTable](ctx, map[string]any{"ID": int64(2)})
	require.NoError(t, err)
	n, _ := res.RowsAffected()
	assert.Equal(t, int64(1), n)

	// Fetch should exclude soft-deleted
	results, err := psql.Fetch[SoftDelTable](ctx, nil)
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.NotEqual(t, "delete_me", r.Label)
	}

	// Count should exclude soft-deleted
	cnt, err := psql.Count[SoftDelTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// Get should exclude soft-deleted
	_, err = psql.Get[SoftDelTable](ctx, map[string]any{"ID": int64(2)})
	assert.Error(t, err, "soft-deleted record should not be found by Get")

	// WithDeleted should include soft-deleted
	results, err = psql.Fetch[SoftDelTable](ctx, nil, psql.IncludeDeleted())
	require.NoError(t, err)
	assert.Len(t, results, 3)

	// Count with IncludeDeleted
	cnt, err = psql.Count[SoftDelTable](ctx, nil, psql.IncludeDeleted())
	require.NoError(t, err)
	assert.Equal(t, 3, cnt)

	// Verify the soft-deleted record has a DeletedAt timestamp
	obj, err := psql.Get[SoftDelTable](ctx, map[string]any{"ID": int64(2)}, psql.IncludeDeleted())
	require.NoError(t, err)
	assert.NotNil(t, obj.DeletedAt)
	assert.Equal(t, "delete_me", obj.Label)

	// Restore
	_, err = psql.Restore[SoftDelTable](ctx, map[string]any{"ID": int64(2)})
	require.NoError(t, err)

	// Should be visible again
	cnt, err = psql.Count[SoftDelTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, cnt)

	// ForceDelete should hard-delete
	_, err = psql.ForceDelete[SoftDelTable](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)

	// Gone even with IncludeDeleted
	cnt, err = psql.Count[SoftDelTable](ctx, nil, psql.IncludeDeleted())
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	t.Run("DoubleDelete", func(t *testing.T) {
		// Re-insert for double delete test
		require.NoError(t, psql.Insert(ctx, &SoftDelTable{ID: 10, Label: "once"}))

		// First delete
		res, err := psql.Delete[SoftDelTable](ctx, map[string]any{"ID": int64(10)})
		require.NoError(t, err)
		n, _ := res.RowsAffected()
		assert.Equal(t, int64(1), n)

		// Second delete should affect 0 rows (already soft-deleted)
		res, err = psql.Delete[SoftDelTable](ctx, map[string]any{"ID": int64(10)})
		require.NoError(t, err)
		n, _ = res.RowsAffected()
		assert.Equal(t, int64(0), n)
	})

	_ = psql.Q("DROP TABLE IF EXISTS \"test_softdel\"").Exec(ctx)
}
