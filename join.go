package psql

import "context"

// Join represents a SQL JOIN clause with a table, condition, type, and optional alias.
type Join struct {
	Table     string
	Condition string // condition for join
	Type      string // LEFT|INNER|RIGHT
	Alias     string // if any
}

// JoinTable returns the formatted table name using the namer if available
func (j *Join) JoinTable(ctx context.Context) string {
	be := GetBackend(ctx)
	if be != nil && be.namer != nil {
		return be.namer.JoinTableName(j.Table)
	}
	return FormatTableName(j.Table)
}
