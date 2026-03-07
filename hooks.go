package psql

import "context"

// BeforeInsertHook is called before an INSERT operation.
type BeforeInsertHook interface {
	BeforeInsert(ctx context.Context) error
}

// AfterInsertHook is called after a successful INSERT operation.
type AfterInsertHook interface {
	AfterInsert(ctx context.Context) error
}

// BeforeUpdateHook is called before an UPDATE operation.
type BeforeUpdateHook interface {
	BeforeUpdate(ctx context.Context) error
}

// AfterUpdateHook is called after a successful UPDATE operation.
type AfterUpdateHook interface {
	AfterUpdate(ctx context.Context) error
}

// BeforeSaveHook is called before INSERT, UPDATE, and REPLACE operations.
type BeforeSaveHook interface {
	BeforeSave(ctx context.Context) error
}

// AfterSaveHook is called after successful INSERT, UPDATE, and REPLACE operations.
type AfterSaveHook interface {
	AfterSave(ctx context.Context) error
}

// AfterScanHook is called after a row is scanned from the database during fetch operations.
type AfterScanHook interface {
	AfterScan(ctx context.Context) error
}
