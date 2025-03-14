package psql

import "context"

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
