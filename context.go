package psql

import (
	"context"
	"database/sql"
)

type ctxData int

const (
	ctxDataObj ctxData = iota
	ctxValueObjFetch
)

type ctxValueObj struct {
	context.Context
	obj any
}

func (c *ctxValueObj) Value(v any) any {
	if v == ctxDataObj {
		return c.obj
	}
	if v == ctxValueObjFetch {
		return c
	}
	return c.Context.Value(v)
}

func ContextDB(ctx context.Context, db *sql.DB) context.Context {
	return &ctxValueObj{ctx, db}
}

func ContextConn(ctx context.Context, conn *sql.Conn) context.Context {
	return &ctxValueObj{ctx, conn}
}

func ContextTx(ctx context.Context, tx *TxProxy) context.Context {
	return &ctxValueObj{ctx, tx}
}

// Tx can be used to run a function inside a sql transaction for isolation/etc
func Tx(ctx context.Context, cb func(ctx context.Context) error) error {
	tx, err := BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ctx = ContextTx(ctx, tx)
	err = cb(ctx)
	if err == nil {
		return tx.Commit()
	}
	return err
}

func BeginTx(ctx context.Context, opts *sql.TxOptions) (*TxProxy, error) {
	obj := ctx.Value(ctxDataObj)
	if obj == nil {
		return newTxCtrl(db.BeginTx(ctx, opts))
	}

	switch o := obj.(type) {
	case *sql.Conn:
		return newTxCtrl(o.BeginTx(ctx, opts))
	case *sql.DB:
		return newTxCtrl(o.BeginTx(ctx, opts))
	case *TxProxy:
		return o.BeginTx(ctx, opts)
	case interface {
		BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
	}:
		return newTxCtrl(o.BeginTx(ctx, opts))
	default:
		return newTxCtrl(db.BeginTx(ctx, opts))
	}
}

// EscapeTx allows obtaining the context underlying a current transaction, this can be useful
// if a query needs to be run outside of a transaction (for example to log something, etc)
func EscapeTx(ctx context.Context) (context.Context, bool) {
	for {
		obj := ctx.Value(ctxValueObjFetch)
		if obj == nil {
			// no parent object, just return the same ctx
			return ctx, false
		}
		objV := obj.(ctxValueObj)

		switch objV.obj.(type) {
		case *sql.Tx:
			// we reached the point we wanted
			return objV.Context, true
		}

		// we need to go deeper
		ctx = objV.Context
	}
}

func doExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	obj := ctx.Value(ctxDataObj)
	if obj == nil {
		return db.ExecContext(ctx, query, args...)
	}

	switch o := obj.(type) {
	case *sql.Tx:
		debugLog(ctx, "Exec on tx: %s %v", query, args)
		return o.ExecContext(ctx, query, args...)
	case *TxProxy:
		debugLog(ctx, "Exec on tx proxy: %s %v", query, args)
		return o.ExecContext(ctx, query, args...)
	case *sql.Conn:
		debugLog(ctx, "Exec on conn: %s %v", query, args)
		return o.ExecContext(ctx, query, args...)
	case *sql.DB:
		debugLog(ctx, "Exec on DB: %s %v", query, args)
		return o.ExecContext(ctx, query, args...)
	case interface {
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}:
		debugLog(ctx, "Exec on %T: %s %v", o, query, args)
		return o.ExecContext(ctx, query, args...)
	default:
		// unknown object, fallback to standard
		debugLog(ctx, "Exec on DB because %T is unknown: %s %v", o, query, args)
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
		debugLog(ctx, "Query on tx: %s %v", query, args)
		return o.QueryContext(ctx, query, args...)
	case *TxProxy:
		debugLog(ctx, "Query on tx proxy: %s %v", query, args)
		return o.QueryContext(ctx, query, args...)
	case *sql.Conn:
		debugLog(ctx, "Query on conn: %s %v", query, args)
		return o.QueryContext(ctx, query, args...)
	case *sql.DB:
		debugLog(ctx, "Query on db: %s %v", query, args)
		return o.QueryContext(ctx, query, args...)
	case interface {
		QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	}:
		debugLog(ctx, "Query on %T: %s %v", o, query, args)
		return o.QueryContext(ctx, query, args...)
	default:
		debugLog(ctx, "Query db because %T is unknown: %s %v", o, query, args)
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
		debugLog(ctx, "Prepare on tx: %s", query)
		return o.PrepareContext(ctx, query)
	case *TxProxy:
		debugLog(ctx, "Prepare on tx proxy: %s", query)
		return o.PrepareContext(ctx, query)
	case *sql.Conn:
		debugLog(ctx, "Prepare on conn: %s", query)
		return o.PrepareContext(ctx, query)
	case *sql.DB:
		debugLog(ctx, "Prepare on DB: %s", query)
		return o.PrepareContext(ctx, query)
	case interface {
		PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	}:
		debugLog(ctx, "Prepare on %T: %s", o, query)
		return o.PrepareContext(ctx, query)
	default:
		// unknown object, fallback to standard
		debugLog(ctx, "Prepare on DB because %T is unknown: %s", o, query)
		return db.PrepareContext(ctx, query)
	}
}
