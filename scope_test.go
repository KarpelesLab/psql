package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScopeApply(t *testing.T) {
	ctx := context.Background()

	active := psql.Scope(func(q *psql.QueryBuilder) *psql.QueryBuilder {
		return q.Where(map[string]any{"Status": "active"})
	})
	limited := psql.Scope(func(q *psql.QueryBuilder) *psql.QueryBuilder {
		return q.Limit(5)
	})

	query := psql.B().Select("*").From("users").Apply(active, limited)
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "Status")
	assert.Contains(t, sql, "active")
	assert.Contains(t, sql, "LIMIT 5")
}

func TestScopeWithFetchOptions(t *testing.T) {
	// Verify WithScope creates proper FetchOptions
	scope := psql.Scope(func(q *psql.QueryBuilder) *psql.QueryBuilder {
		return q.Where(map[string]any{"Visible": true})
	})
	opts := psql.WithScope(scope)
	require.NotNil(t, opts)
	assert.Len(t, opts.Scopes, 1)
}

type ScopeTestTable struct {
	psql.Name `sql:"test_scope"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
	Status    string `sql:",type=VARCHAR,size=32"`
}

func TestScopeIntegration(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_scope\"").Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &ScopeTestTable{ID: 1, Label: "a", Status: "active"}))
	require.NoError(t, psql.Insert(ctx, &ScopeTestTable{ID: 2, Label: "b", Status: "inactive"}))
	require.NoError(t, psql.Insert(ctx, &ScopeTestTable{ID: 3, Label: "c", Status: "active"}))

	active := psql.Scope(func(q *psql.QueryBuilder) *psql.QueryBuilder {
		return q.Where(map[string]any{"Status": "active"})
	})

	// Fetch with scope
	results, err := psql.Fetch[ScopeTestTable](ctx, nil, psql.WithScope(active))
	require.NoError(t, err)
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, "active", r.Status)
	}

	// Count with scope
	cnt, err := psql.Count[ScopeTestTable](ctx, nil, psql.WithScope(active))
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// Scope combined with where
	results, err = psql.Fetch[ScopeTestTable](ctx, map[string]any{"Label": "a"}, psql.WithScope(active))
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "a", results[0].Label)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_scope\"").Exec(ctx)
}
