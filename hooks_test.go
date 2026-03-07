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

func TestHooksInsertIgnore(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	obj := &HookTestObj{ID: 1, Label: "ignore"}
	err := psql.InsertIgnore(ctx, obj)
	require.NoError(t, err)

	assert.Equal(t, []string{"before_save", "before_insert", "after_insert", "after_save"}, obj.HookLog)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}

func TestHooksBatchInsert(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	obj1 := &HookTestObj{ID: 1, Label: "first"}
	obj2 := &HookTestObj{ID: 2, Label: "second"}
	err := psql.Insert(ctx, obj1, obj2)
	require.NoError(t, err)

	// Both objects should have hooks called
	assert.Equal(t, []string{"before_save", "before_insert", "after_insert", "after_save"}, obj1.HookLog)
	assert.Equal(t, []string{"before_save", "before_insert", "after_insert", "after_save"}, obj2.HookLog)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}

// Test BeforeUpdate error prevents the update
type HookUpdateErrorObj struct {
	psql.Name `sql:"test_hook_upd_err"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

var errUpdateFail = errors.New("update blocked")

func (h *HookUpdateErrorObj) BeforeUpdate(ctx context.Context) error {
	return errUpdateFail
}

func TestHookErrorPreventsUpdate(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_upd_err"`).Exec(ctx)

	obj := &HookUpdateErrorObj{ID: 1, Label: "original"}
	require.NoError(t, psql.Insert(ctx, obj))

	obj.Label = "changed"
	err := psql.Update(ctx, obj)
	assert.ErrorIs(t, err, errUpdateFail)

	// Verify the value was NOT updated in the database
	fetched, err := psql.Get[HookUpdateErrorObj](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Equal(t, "original", fetched.Label)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_upd_err"`).Exec(ctx)
}

// Test AfterScan error prevents object return
type HookScanErrorObj struct {
	psql.Name `sql:"test_hook_scan_err"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

var errScanFail = errors.New("scan blocked")

func (h *HookScanErrorObj) AfterScan(ctx context.Context) error {
	return errScanFail
}

func TestHookAfterScanError(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_scan_err"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &HookScanErrorObj{ID: 1, Label: "test"}))

	// Get should fail with the scan hook error
	_, err := psql.Get[HookScanErrorObj](ctx, map[string]any{"ID": int64(1)})
	assert.ErrorIs(t, err, errScanFail)

	// Fetch should also fail
	_, err = psql.Fetch[HookScanErrorObj](ctx, nil)
	assert.ErrorIs(t, err, errScanFail)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_scan_err"`).Exec(ctx)
}

// Test that BeforeInsert can modify the object before it's saved
type HookMutateObj struct {
	psql.Name `sql:"test_hook_mutate"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

func (h *HookMutateObj) BeforeInsert(ctx context.Context) error {
	if h.Label == "" {
		h.Label = "default_label"
	}
	return nil
}

func TestHookMutatesObject(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_mutate"`).Exec(ctx)

	obj := &HookMutateObj{ID: 1} // Label is empty
	require.NoError(t, psql.Insert(ctx, obj))

	// The hook should have set the default
	assert.Equal(t, "default_label", obj.Label)

	// And it should be persisted
	fetched, err := psql.Get[HookMutateObj](ctx, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Equal(t, "default_label", fetched.Label)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_mutate"`).Exec(ctx)
}

func TestHooksAfterScanFetchOne(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)

	require.NoError(t, psql.Insert(ctx, &HookTestObj{ID: 1, Label: "test"}))

	var obj HookTestObj
	err := psql.FetchOne(ctx, &obj, map[string]any{"ID": int64(1)})
	require.NoError(t, err)
	assert.Contains(t, obj.HookLog, "after_scan")

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hooks"`).Exec(ctx)
}

// Test BeforeSave error prevents replace
type HookSaveErrorObj struct {
	psql.Name `sql:"test_hook_save_err"`
	ID        int64  `sql:",key=PRIMARY"`
	Label     string `sql:",type=VARCHAR,size=128"`
}

var errSaveFail = errors.New("save blocked")

func (h *HookSaveErrorObj) BeforeSave(ctx context.Context) error {
	return errSaveFail
}

func TestHookBeforeSaveErrorPreventsReplace(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())
	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_save_err"`).Exec(ctx)

	obj := &HookSaveErrorObj{ID: 1, Label: "test"}
	err := psql.Replace(ctx, obj)
	assert.ErrorIs(t, err, errSaveFail)

	cnt, err := psql.Count[HookSaveErrorObj](ctx, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, cnt)

	_ = psql.Q(`DROP TABLE IF EXISTS "test_hook_save_err"`).Exec(ctx)
}
