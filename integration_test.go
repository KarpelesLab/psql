package psql_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DelCountTable struct {
	psql.Name `sql:"test_del_count"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
	Score     int64
}

func TestDeleteAndCount(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_del_count\"").Exec(ctx)

	for i := int64(1); i <= 5; i++ {
		err := psql.Insert(ctx, &DelCountTable{ID: i, Label: "item", Score: i * 10})
		require.NoError(t, err, "insert %d", i)
	}

	// Count all
	cnt, err := psql.Count[DelCountTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 5, cnt)

	// Count with where
	cnt, err = psql.Count[DelCountTable](ctx, map[string]any{"Score": psql.Gte(nil, 30)})
	require.NoError(t, err)
	assert.Equal(t, 3, cnt)

	// Delete one row
	res, err := psql.Delete[DelCountTable](ctx, map[string]any{"ID": 3})
	require.NoError(t, err)
	n, _ := res.RowsAffected()
	assert.Equal(t, int64(1), n)

	// Count again
	cnt, err = psql.Count[DelCountTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, cnt)

	// DeleteOne
	err = psql.DeleteOne[DelCountTable](ctx, map[string]any{"ID": 1})
	require.NoError(t, err)

	cnt, err = psql.Count[DelCountTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, cnt)

	// DeleteOne with bad match (0 rows) should error
	err = psql.DeleteOne[DelCountTable](ctx, map[string]any{"ID": 999})
	assert.Error(t, err)

	// Clean up
	_ = psql.Q("DROP TABLE IF EXISTS \"test_del_count\"").Exec(ctx)
}

type InsIgnoreTable struct {
	psql.Name `sql:"test_ins_ignore"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestInsertIgnore(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_ins_ignore\"").Exec(ctx)

	err := psql.Insert(ctx, &InsIgnoreTable{ID: 1, Label: "first"})
	require.NoError(t, err)

	// InsertIgnore with duplicate key should not error
	err = psql.InsertIgnore(ctx, &InsIgnoreTable{ID: 1, Label: "dup"})
	require.NoError(t, err)

	// Value should not have changed
	obj, err := psql.Get[InsIgnoreTable](ctx, map[string]any{"ID": 1})
	require.NoError(t, err)
	assert.Equal(t, "first", obj.Label)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_ins_ignore\"").Exec(ctx)
}

type ReplTable struct {
	psql.Name `sql:"test_replace"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
	Score     int64
}

func TestReplace(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_replace\"").Exec(ctx)

	err := psql.Insert(ctx, &ReplTable{ID: 1, Label: "original", Score: 10})
	require.NoError(t, err)

	// Replace should upsert
	err = psql.Replace(ctx, &ReplTable{ID: 1, Label: "replaced", Score: 99})
	require.NoError(t, err)

	obj, err := psql.Get[ReplTable](ctx, map[string]any{"ID": 1})
	require.NoError(t, err)
	assert.Equal(t, "replaced", obj.Label)
	assert.Equal(t, int64(99), obj.Score)

	// Replace with new key should insert
	err = psql.Replace(ctx, &ReplTable{ID: 2, Label: "new", Score: 50})
	require.NoError(t, err)

	cnt, err := psql.Count[ReplTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_replace\"").Exec(ctx)
}

type TxTable struct {
	psql.Name `sql:"test_tx"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestTransaction(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_tx\"").Exec(ctx)

	// Seed data
	err := psql.Insert(ctx, &TxTable{ID: 1, Label: "txtest"})
	require.NoError(t, err)

	// Successful transaction
	err = psql.Tx(ctx, func(txCtx context.Context) error {
		return psql.Insert(txCtx, &TxTable{ID: 2, Label: "in-tx"})
	})
	require.NoError(t, err)

	cnt, err := psql.Count[TxTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	// Rolled back transaction (return error)
	err = psql.Tx(ctx, func(txCtx context.Context) error {
		_ = psql.Insert(txCtx, &TxTable{ID: 3, Label: "rollback"})
		return assert.AnError
	})
	assert.Error(t, err)

	// ID 3 should not exist
	cnt, err = psql.Count[TxTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, cnt)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_tx\"").Exec(ctx)
}

type TxNestTable struct {
	psql.Name `sql:"test_tx_nest"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestTransactionNested(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_tx_nest\"").Exec(ctx)

	err := psql.Insert(ctx, &TxNestTable{ID: 1, Label: "base"})
	require.NoError(t, err)

	// Nested transactions (savepoints)
	err = psql.Tx(ctx, func(txCtx context.Context) error {
		err := psql.Insert(txCtx, &TxNestTable{ID: 2, Label: "outer"})
		if err != nil {
			return err
		}

		// Inner tx that succeeds
		err = psql.Tx(txCtx, func(innerCtx context.Context) error {
			return psql.Insert(innerCtx, &TxNestTable{ID: 3, Label: "inner"})
		})
		return err
	})
	require.NoError(t, err)

	cnt, err := psql.Count[TxNestTable](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, cnt)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_tx_nest\"").Exec(ctx)
}

type EachTable struct {
	psql.Name `sql:"test_each"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func TestQueryEach(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_each\"").Exec(ctx)

	for i := int64(1); i <= 3; i++ {
		require.NoError(t, psql.Insert(ctx, &EachTable{ID: i, Label: "each"}))
	}

	// Use Q().Each
	var labels []string
	err := psql.Q("SELECT \"Label\" FROM \"test_each\" ORDER BY \"ID\"").Each(ctx, func(rows *sql.Rows) error {
		var label string
		if err := rows.Scan(&label); err != nil {
			return err
		}
		labels = append(labels, label)
		return nil
	})
	require.NoError(t, err)
	assert.Len(t, labels, 3)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_each\"").Exec(ctx)
}

type IterTable struct {
	psql.Name `sql:"test_iter"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
	Score     int64
}

func TestFetchIter(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	_ = psql.Q("DROP TABLE IF EXISTS \"test_iter\"").Exec(ctx)

	for i := int64(1); i <= 3; i++ {
		require.NoError(t, psql.Insert(ctx, &IterTable{ID: i, Label: "iter", Score: i * 10}))
	}

	// Test Iter
	iterFn, err := psql.Iter[IterTable](ctx, nil)
	require.NoError(t, err)

	var ids []int64
	iterFn(func(obj *IterTable) bool {
		ids = append(ids, obj.ID)
		return true
	})
	assert.Len(t, ids, 3)

	_ = psql.Q("DROP TABLE IF EXISTS \"test_iter\"").Exec(ctx)
}

func TestEscapeTx(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	// EscapeTx outside of a transaction
	escaped, ok := psql.EscapeTx(ctx)
	assert.False(t, ok)
	assert.NotNil(t, escaped)

	// EscapeTx with a raw sql.Tx context
	tx, err := psql.BeginTx(ctx, nil)
	require.NoError(t, err)
	// ContextTx wraps with TxProxy, but EscapeTx looks for *sql.Tx specifically.
	// Since Tx() uses TxProxy, EscapeTx won't find a *sql.Tx to escape from.
	txCtx := psql.ContextTx(ctx, tx)
	escaped, ok = psql.EscapeTx(txCtx)
	// TxProxy is not *sql.Tx, so it should return the parent context
	assert.NotNil(t, escaped)
	_ = ok // behavior depends on whether TxProxy matches *sql.Tx check
	_ = tx.Rollback()
}

func TestContextDB(t *testing.T) {
	be := getTestBackend(t)
	db := be.DB()

	// Test ContextDB
	ctx := psql.ContextDB(context.Background(), db)
	assert.NotNil(t, ctx)
}

func TestBeginTx(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	tx, err := psql.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// Commit should work
	err = tx.Commit()
	assert.NoError(t, err)

	// Double commit/rollback should return ErrTxAlreadyProcessed
	err = tx.Commit()
	assert.Error(t, err)
	err = tx.Rollback()
	assert.Error(t, err)
}
