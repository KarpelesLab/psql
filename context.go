package psql

import (
	"context"
	"database/sql"
)

type ctxData int

const (
	ctxDataObj ctxData = iota
)

type ctxValueObj struct {
	context.Context
	obj any
}

func (c *ctxValueObj) Value(v any) any {
	if v == ctxDataObj {
		return c.obj
	}
	return c.Context.Value(v)
}

func ContextDB(ctx context.Context, db *sql.DB) context.Context {
	return &ctxValueObj{ctx, db}
}

func ContextConn(ctx context.Context, conn *sql.Conn) context.Context {
	return &ctxValueObj{ctx, conn}
}

func ContextTx(ctx context.Context, tx *sql.Tx) context.Context {
	return &ctxValueObj{ctx, tx}
}

func doExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	obj := ctx.Value(ctxDataObj)
	if obj == nil {
		return db.ExecContext(ctx, query, args...)
	}

	switch o := obj.(type) {
	case *sql.Tx:
		return o.ExecContext(ctx, query, args...)
	case *sql.Conn:
		return o.ExecContext(ctx, query, args...)
	case *sql.DB:
		return o.ExecContext(ctx, query, args...)
	case interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}:
		return o.ExecContext(ctx, query, args...)
	default:
		// unknown object, fallback to standard
		return db.ExecContext(ctx, query, args...)
	}
}

func doQueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	obj := ctx.Value(ctxDataObj)
	if obj == nil {
		return db.QueryContext(ctx, query, args...)
	}

	switch o := obj.(type) {
	case *sql.Tx:
		return o.QueryContext(ctx, query, args...)
	case *sql.Conn:
		return o.QueryContext(ctx, query, args...)
	case *sql.DB:
		return o.QueryContext(ctx, query, args...)
	case interface {
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	}:
		return o.QueryContext(ctx, query, args...)
	default:
		// unknown object, fallback to standard
		return db.QueryContext(ctx, query, args...)
	}
}

func doPrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	obj := ctx.Value(ctxDataObj)
	if obj == nil {
		return db.PrepareContext(ctx, query)
	}

	switch o := obj.(type) {
	case *sql.Tx:
		return o.PrepareContext(ctx, query)
	case *sql.Conn:
		return o.PrepareContext(ctx, query)
	case *sql.DB:
		return o.PrepareContext(ctx, query)
	case interface {
		PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	}:
		return o.PrepareContext(ctx, query)
	default:
		// unknown object, fallback to standard
		return db.PrepareContext(ctx, query)
	}
}
