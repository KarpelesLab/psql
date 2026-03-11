package psql_test

import (
	"context"
	"testing"
	"time"

	"github.com/portablesql/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// === Now() tests ===

func TestNowDefault(t *testing.T) {
	ctx := context.Background()

	query := psql.B().Select(psql.Now()).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW()")
}

func TestNowMySQL(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	query := psql.B().Select(psql.Now()).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW()")
}

func TestNowPostgreSQL(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select(psql.Now()).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW()")
}

func TestNowSQLite(t *testing.T) {
	ctx := ctxForEngine(psql.EngineSQLite)

	query := psql.B().Select(psql.Now()).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "CURRENT_TIMESTAMP")
	assert.NotContains(t, sql, "NOW()")
}

// === DateAdd() engine-specific rendering ===

func TestDateAddMySQL(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateAdd(psql.Now(), -24*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() - INTERVAL 1 DAY")
}

func TestDateAddPostgreSQL(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateAdd(psql.Now(), -24*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() - INTERVAL '1 day'")
}

func TestDateAddSQLite(t *testing.T) {
	ctx := ctxForEngine(psql.EngineSQLite)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateAdd(psql.Now(), -24*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "datetime(CURRENT_TIMESTAMP,'-1 days')")
}

// === DateAdd with different units ===

func TestDateAddSeconds(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	query := psql.B().Select().From("t").
		Where(psql.Gt(psql.F("ts"), psql.DateAdd(psql.F("start_time"), 30*time.Second)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"start_time" + INTERVAL 30 SECOND`)
}

func TestDateAddMinutes(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select().From("t").
		Where(psql.Lt(psql.F("expires_at"), psql.DateAdd(psql.Now(), 15*time.Minute)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() + INTERVAL '15 minute'")
}

func TestDateAddHours(t *testing.T) {
	ctx := ctxForEngine(psql.EngineSQLite)

	query := psql.B().Select().From("sessions").
		Where(psql.Lt(psql.F("last_seen"), psql.DateAdd(psql.Now(), -2*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "datetime(CURRENT_TIMESTAMP,'-2 hours')")
}

// === DateSub tests ===

func TestDateSubMySQL(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateSub(psql.Now(), 7*24*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() - INTERVAL 7 DAY")
}

func TestDateSubPostgreSQL(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateSub(psql.Now(), 30*time.Minute)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() - INTERVAL '30 minute'")
}

func TestDateSubSQLite(t *testing.T) {
	ctx := ctxForEngine(psql.EngineSQLite)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateSub(psql.Now(), 3600*time.Second)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "datetime(CURRENT_TIMESTAMP,'-1 hours')")
}

// === DateAdd with field expressions ===

func TestDateAddFieldMySQL(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	query := psql.B().Select().From("tasks").
		Where(psql.Lt(psql.Now(), psql.DateAdd(psql.F("deadline"), -1*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"deadline" - INTERVAL 1 HOUR`)
}

func TestDateAddFieldPostgreSQL(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select().From("tasks").
		Where(psql.Lt(psql.Now(), psql.DateAdd(psql.F("deadline"), -1*time.Hour)))
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"deadline" - INTERVAL '1 hour'`)
}

func TestDateAddFieldSQLite(t *testing.T) {
	ctx := ctxForEngine(psql.EngineSQLite)

	query := psql.B().Select(psql.DateAdd(psql.F("created_at"), 48*time.Hour)).From("events")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `datetime("created_at",'+2 days')`)
}

// === RenderArgs tests ===

func TestDateAddRenderArgsPostgreSQL(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateAdd(psql.Now(), -24*time.Hour)))
	sql, args, err := query.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() - INTERVAL '1 day'")
	assert.Len(t, args, 0) // no bind params for NOW() or INTERVAL
}

func TestDateAddRenderArgsSQLite(t *testing.T) {
	ctx := ctxForEngine(psql.EngineSQLite)

	query := psql.B().Select().From("events").
		Where(psql.Gt(psql.F("created_at"), psql.DateAdd(psql.Now(), -24*time.Hour)))
	sql, args, err := query.RenderArgs(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "datetime(CURRENT_TIMESTAMP,'-1 days')")
	assert.Len(t, args, 0)
}

// === Edge cases ===

func TestDateAddZeroDuration(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	query := psql.B().Select(psql.DateAdd(psql.F("ts"), 0)).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"ts" + INTERVAL 0 SECOND`)
}

func TestDateAddOddSeconds(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	// 90 seconds doesn't divide evenly into minutes
	query := psql.B().Select(psql.DateAdd(psql.F("ts"), 90*time.Second)).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"ts" + INTERVAL '90 second'`)
}

func TestDateAddDefault(t *testing.T) {
	// No engine context → default (MySQL-like) rendering
	ctx := context.Background()

	query := psql.B().Select(psql.DateAdd(psql.Now(), 5*time.Minute)).From("t")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, "NOW() + INTERVAL 5 MINUTE")
}

// === Composability: DateAdd in WHERE, SELECT, SET ===

func TestDateAddInWhere(t *testing.T) {
	ctx := ctxForEngine(psql.EngineMySQL)

	// "created_at > NOW() - INTERVAL 1 DAY" — the portable version
	query := psql.B().Select("status", psql.Raw("COUNT(*) as count")).
		From("users").
		Where(psql.Gt(psql.F("created_at"), psql.DateSub(psql.Now(), 24*time.Hour))).
		GroupByFields("status")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"created_at">NOW() - INTERVAL 1 DAY`)
}

func TestDateAddInSelect(t *testing.T) {
	ctx := ctxForEngine(psql.EnginePostgreSQL)

	query := psql.B().Select("id", psql.DateAdd(psql.F("created_at"), 7*24*time.Hour)).From("events")
	sql, err := query.Render(ctx)
	require.NoError(t, err)
	assert.Contains(t, sql, `"created_at" + INTERVAL '7 day'`)
}
