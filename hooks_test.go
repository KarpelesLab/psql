package psql_test

import (
	"context"
	"errors"
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type HookTestObj struct {
	psql.Name `sql:"test_hooks"`
	ID        int64    `sql:",key=PRIMARY"`
	Label     string   `sql:",type=VARCHAR,size=128"`
	HookLog   []string `sql:"-"`
}

func (h *HookTestObj) BeforeSave(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "before_save")
	return nil
}

func (h *HookTestObj) BeforeInsert(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "before_insert")
	return nil
}

func (h *HookTestObj) AfterInsert(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "after_insert")
	return nil
}

func (h *HookTestObj) AfterSave(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "after_save")
	return nil
}

func (h *HookTestObj) BeforeUpdate(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "before_update")
	return nil
}

func (h *HookTestObj) AfterUpdate(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "after_update")
	return nil
}

func (h *HookTestObj) AfterScan(ctx context.Context) error {
	h.HookLog = append(h.HookLog, "after_scan")
	return nil
}

func TestHooksInsert(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	obj := &HookTestObj{ID: 1, Label: "test"}
	err := psql.Insert(ctx, obj)
	require.NoError(t, err)

	assert.Equal(t, []string{"before_save", "before_insert", "after_insert", "after_save"}, obj.HookLog)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}

func TestHooksUpdate(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	obj := &HookTestObj{ID: 1, Label: "test"}
	require.NoError(t, psql.Insert(ctx, obj))

	obj.HookLog = nil
	obj.Label = "updated"
	err := psql.Update(ctx, obj)
	require.NoError(t, err)

	assert.Equal(t, []string{"before_save", "before_update", "after_update", "after_save"}, obj.HookLog)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}

func TestHooksAfterScan(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &HookTestObj{ID: 1, Label: "test"}))

	// Get triggers AfterScan
	obj, err := psql.Get[HookTestObj](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Contains(t, obj.HookLog, "after_scan")

	// Fetch triggers AfterScan for each object
	require.NoError(t, psql.Insert(ctx, &HookTestObj{ID: 2, Label: "test2"}))
	objs, err := psql.Fetch[HookTestObj](ctx, nil)
	require.NoError(t, err)
	for _, o := range objs {
		assert.Contains(t, o.HookLog, "after_scan")
	}

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}

// Test that a BeforeInsert error prevents the insert
var errHookFail = errors.New("hook failed")

type HookErrorObj struct {
	psql.Name `sql:"test_hook_error"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func (h *HookErrorObj) BeforeInsert(ctx context.Context) error {
	return errHookFail
}

func TestHookErrorPreventsInsert(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_error"`).Exec(ctx)

	// Force table creation by inserting a successful type first, then check
	// Actually the table is created by t.check() before hooks fire
	obj := &HookErrorObj{ID: 1, Label: "test"}
	err := psql.Insert(ctx, obj)
	assert.ErrorIs(t, err, errHookFail)

	// The row should not exist
	cnt, err := psql.Count[HookErrorObj](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_error"`).Exec(ctx)
}

func TestHooksReplace(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	obj := &HookTestObj{ID: 1, Label: "replace"}
	err := psql.Replace(ctx, obj)
	require.NoError(t, err)

	// Replace fires BeforeSave and AfterSave
	assert.Contains(t, obj.HookLog, "before_save")
	assert.Contains(t, obj.HookLog, "after_save")

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}
