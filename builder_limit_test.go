package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuilderLimitSyntax tests that LIMIT syntax is correct for different database engines
func TestBuilderLimitSyntax(t *testing.T) {
	t.Run("Default LIMIT syntax", func(t *testing.T) {
		// Default context (no backend) should use default dialect
		ctx := context.Background()

		// Test LIMIT without offset
		query := psql.B().Select().From("users").Limit(10)
		sql, err := query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10`, sql)

		// Test LIMIT with offset (default dialect uses MySQL-like syntax)
		query = psql.B().Select().From("users").Limit(10, 20)
		sql, err = query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10, 20`, sql)
	})

	t.Run("SQLite LIMIT syntax", func(t *testing.T) {
		be := getTestBackend(t)
		if be.Engine() != psql.EngineSQLite {
			t.Skip("Test only applicable for SQLite")
		}

		ctx := be.Plug(context.Background())

		// Test LIMIT without offset
		query := psql.B().Select().From("users").Limit(10)
		sql, err := query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10`, sql)

		// Test LIMIT with offset (SQLite style: LIMIT x OFFSET y)
		query = psql.B().Select().From("users").Limit(10, 20)
		sql, err = query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10 OFFSET 20`, sql)
	})

	t.Run("PostgreSQL LIMIT syntax", func(t *testing.T) {
		be := getTestBackend(t)
		if be.Engine() != psql.EnginePostgreSQL {
			t.Skip("Test only applicable for PostgreSQL")
		}

		ctx := be.Plug(context.Background())

		// Test LIMIT without offset
		query := psql.B().Select().From("users").Limit(10)
		sql, err := query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10`, sql)

		// Test LIMIT with offset (PostgreSQL style: LIMIT x OFFSET y)
		query = psql.B().Select().From("users").Limit(10, 20)
		sql, err = query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10 OFFSET 20`, sql)
	})
}

// TestBuilderLimitBackwardCompatibility ensures default syntax still works
func TestBuilderLimitBackwardCompatibility(t *testing.T) {
	// Default context (no backend) should use default LIMIT syntax
	ctx := context.Background()

	query := psql.B().Select().From("products").Limit(50, 100)
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "products" LIMIT 50, 100`, sql)
}
