package psql

import (
	"context"
	"fmt"
	"log/slog"
)

// check will run CheckStructure if it hasn't been run yet on this connection
func (t *TableMeta[T]) check(ctx context.Context) {
	be := GetBackend(ctx)
	if be.checkedOnce(t.typ) {
		return
	}

	d := be.Engine().dialect()
	if sc, ok := d.(SchemaChecker); ok {
		err := sc.CheckStructure(ctx, be, t)
		if err != nil {
			slog.ErrorContext(ctx, fmt.Sprintf("psql: failed to check table %s: %s", t.table, err), "event", "psql:table:check_error", "psql.table", t.table)
		}
	}
}
