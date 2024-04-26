package psql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
)

type TxProxy struct {
	*sql.Tx
	ctrl  *txController
	depth int
	once  uint64
}

type txController struct {
	depth int
	lk    sync.Mutex
	tx    *sql.Tx
}

func (t *TxProxy) BeginTx(ctx context.Context, opts *sql.TxOptions) (*TxProxy, error) {
	return t.ctrl.beginSubTx(ctx, opts)
}

func newTxCtrl(tx *sql.Tx, err error) (*TxProxy, error) {
	if err != nil {
		return nil, err
	}

	ctrl := &txController{
		tx: tx,
	}
	res := &TxProxy{
		Tx:   tx,
		ctrl: ctrl,
	}

	return res, nil
}

func (c *txController) beginSubTx(ctx context.Context, opts *sql.TxOptions) (*TxProxy, error) {
	c.lk.Lock()
	defer c.lk.Unlock()

	c.depth += 1

	// create checkpoint
	_, err := c.tx.ExecContext(ctx, fmt.Sprintf("SAVEPOINT L%d", c.depth))
	if err != nil {
		return nil, err
	}

	return &TxProxy{
		Tx:    c.tx,
		ctrl:  c,
		depth: c.depth,
	}, nil
}

func (tx *TxProxy) Commit() error {
	if atomic.AddUint64(&tx.once, 1) != 1 {
		return ErrTxAlreadyProcessed
	}

	return tx.ctrl.commit(tx.depth)
}

func (c *txController) commit(depth int) error {
	c.lk.Lock()
	defer c.lk.Unlock()

	if c.depth != depth {
		// bad sequence
		return fmt.Errorf("invalid depth in committed transaction, expected %d but got %d", c.depth, depth)
	}

	if c.depth > 0 {
		c.depth -= 1
		return nil
	}

	// actually commit
	return c.tx.Commit()
}

func (tx *TxProxy) Rollback() error {
	if atomic.AddUint64(&tx.once, 1) != 1 {
		return ErrTxAlreadyProcessed
	}

	return tx.ctrl.rollback(tx.depth)
}

func (c *txController) rollback(depth int) error {
	c.lk.Lock()
	defer c.lk.Unlock()

	if c.depth != depth {
		// bad sequence
		return fmt.Errorf("invalid depth in committed transaction, expected %d but got %d", c.depth, depth)
	}

	if c.depth > 0 {
		_, err := c.tx.Exec(fmt.Sprintf("ROLLBACK TO L%d", c.depth))
		c.depth -= 1
		return err
	}

	// full rollback
	return c.tx.Rollback()
}
