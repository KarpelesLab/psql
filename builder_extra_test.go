package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuilderInsertRender(t *testing.T) {
	ctx := context.Background()

	// INSERT uses Into() which needs EscapeTableable, so test via Table()
	q := &psql.QueryBuilder{Query: "INSERT"}
	q.Table("users")
	q.FieldsSet = append(q.FieldsSet, map[string]any{"name": "test"})
	sql, err := q.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "INSERT")
	assert.Contains(t, sql, "INTO")
	assert.Contains(t, sql, `"users"`)
	assert.Contains(t, sql, "SET")
}

func TestBuilderInsertIgnoreRender(t *testing.T) {
	ctx := context.Background()

	q := &psql.QueryBuilder{Query: "INSERT", InsertIgnore: true}
	q.Table("items")
	q.FieldsSet = append(q.FieldsSet, map[string]any{"id": 1})
	sql, err := q.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "INSERT")
	assert.Contains(t, sql, "IGNORE")
}

func TestBuilderReplaceRender(t *testing.T) {
	ctx := context.Background()

	q := &psql.QueryBuilder{Query: "REPLACE"}
	q.Table("products")
	q.FieldsSet = append(q.FieldsSet, map[string]any{"id": 1, "name": "widget"})
	sql, err := q.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "REPLACE")
	assert.Contains(t, sql, `"products"`)
	assert.Contains(t, sql, "SET")
}

func TestBuilderRenderArgsSelect(t *testing.T) {
	ctx := context.Background()

	query := psql.B().Select().From("users").Where(map[string]any{
		"name":   "alice",
		"status": "active",
	}).Limit(10)
	sql, args, err := query.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "SELECT")
	assert.Contains(t, sql, "LIMIT 10")
	assert.Len(t, args, 2)
}

func TestBuilderRenderArgsUpdate(t *testing.T) {
	ctx := context.Background()

	query := psql.B().Update("items").Set(map[string]any{"price": 9.99}).Where(map[string]any{"id": 42})
	sql, args, err := query.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "UPDATE")
	assert.Contains(t, sql, "SET")
	assert.Contains(t, sql, "WHERE")
	assert.Len(t, args, 2)
}

func TestBuilderRenderArgsDelete(t *testing.T) {
	ctx := context.Background()

	query := psql.B().Delete().From("logs").Where(map[string]any{"age": psql.Gt(nil, 30)})
	sql, args, err := query.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "DELETE FROM")
	assert.Contains(t, sql, "WHERE")
	assert.Len(t, args, 1)
}

func TestBuilderGroupByRenderArgs(t *testing.T) {
	ctx := context.Background()

	query := psql.B().
		Select(psql.F("dept"), psql.Raw("COUNT(1)")).
		From("employees").
		GroupByFields("dept").
		Having(map[string]any{"COUNT(1)": psql.Gt(nil, 5)})

	sql, args, err := query.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "GROUP BY")
	assert.Contains(t, sql, "HAVING")
	assert.Len(t, args, 1)
}

func TestBuilderMultipleOrderBy(t *testing.T) {
	ctx := context.Background()

	query := psql.B().Select().From("users").
		OrderBy(psql.S("created_at", "DESC"), psql.S("name", "ASC"))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "ORDER BY")
	assert.Contains(t, sql, "DESC")
	assert.Contains(t, sql, "ASC")
}

func TestBuilderLimitOneArg(t *testing.T) {
	ctx := context.Background()

	query := psql.B().Select().From("users").Limit(5)
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Equal(t, `SELECT * FROM "users" LIMIT 5`, sql)
}

func TestBuilderForUpdateRenderArgs(t *testing.T) {
	ctx := context.Background()

	q := psql.B().Select().From("users").Where(map[string]any{"id": 1})
	q.ForUpdate = true
	sql, args, err := q.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "FOR UPDATE")
	assert.Len(t, args, 1)
}

func TestBuilderTableMethod(t *testing.T) {
	ctx := context.Background()

	// Table with string
	q1 := psql.B().Select()
	q1.Table("users")
	sql1, err := q1.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql1, `"users"`)
}

func TestBuilderTableUnsupportedType(t *testing.T) {
	ctx := context.Background()

	q := psql.B().Select()
	q.Table(12345) // unsupported type
	_, err := q.Render(ctx)
	assert.Error(t, err)
}

func TestBuilderSelectFields(t *testing.T) {
	ctx := context.Background()

	// Select with string fields (should be treated as field names)
	query := psql.B().Select("id", "name", "email").From("users")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "SELECT")
	assert.Contains(t, sql, "FROM")
}
