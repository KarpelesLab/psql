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
	t.Run("MySQL LIMIT syntax", func(t *testing.T) {
		// Create a MySQL backend context
		be := &psql.Backend{}
		// MySQL is the default engine
		ctx := be.Plug(context.Background())
		
		// Test LIMIT without offset
		query := psql.B().Select().From("users").Limit(10)
		sql, err := query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10`, sql)
		
		// Test LIMIT with offset (MySQL style: LIMIT x, y)
		query = psql.B().Select().From("users").Limit(10, 20)
		sql, err = query.Render(ctx)
		require.NoError(t, err)
		assert.Equal(t, `SELECT * FROM "users" LIMIT 10, 20`, sql)
	})
	
	t.Run("PostgreSQL LIMIT syntax", func(t *testing.T) {
		// Skip if we can't get a PostgreSQL backend
		be, err := psql.LocalTestServer()
		if err != nil {
			t.Skipf("Unable to launch PostgreSQL test server: %s", err)
			return
		}
		
		if be.Engine() != psql.EnginePostgreSQL {
			t.Skip("Test only applicable for PostgreSQL")
			return
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
	
	t.Run("Complex query with LIMIT", func(t *testing.T) {
		// Test for PostgreSQL
		be, err := psql.LocalTestServer()
		if err != nil {
			t.Skipf("Unable to launch PostgreSQL test server: %s", err)
			return
		}
		
		if be.Engine() != psql.EnginePostgreSQL {
			t.Skip("Test only applicable for PostgreSQL")
			return
		}
		
		ctx := be.Plug(context.Background())
		
		// Complex query with WHERE, ORDER BY, and LIMIT
		query := psql.B().
			Select("id", "name", "email").
			From("users").
			Where(map[string]any{"status": "active"}).
			OrderBy(psql.S("created_at", "DESC")).
			Limit(25, 100)
		
		sql, err := query.Render(ctx)
		require.NoError(t, err)
		assert.Contains(t, sql, "LIMIT 25 OFFSET 100")
		assert.NotContains(t, sql, "LIMIT 25, 100")
	})
	
	t.Run("RenderArgs with LIMIT", func(t *testing.T) {
		// Test that argument rendering also respects the engine
		be, err := psql.LocalTestServer()
		if err != nil {
			t.Skipf("Unable to launch PostgreSQL test server: %s", err)
			return
		}
		
		if be.Engine() != psql.EnginePostgreSQL {
			t.Skip("Test only applicable for PostgreSQL")
			return
		}
		
		ctx := be.Plug(context.Background())
		
		query := psql.B().
			Select().
			From("users").
			Where(map[string]any{"status": "active"}).
			Limit(10, 5)
		
		sql, args, err := query.RenderArgs(ctx)
		require.NoError(t, err)
		assert.Contains(t, sql, "LIMIT 10 OFFSET 5")
		assert.NotContains(t, sql, "LIMIT 10, 5")
		assert.Len(t, args, 1) // Only the WHERE clause should have an argument
		assert.Equal(t, "active", args[0])
	})
}

// TestBuilderLimitBackwardCompatibility ensures MySQL syntax still works as before
func TestBuilderLimitBackwardCompatibility(t *testing.T) {
	// Default context (no backend) should use MySQL syntax for backward compatibility
	ctx := context.Background()
	
	query := psql.B().Select().From("products").Limit(50, 100)
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "products" LIMIT 50, 100`, sql)
}