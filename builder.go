package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// EscapeValueable is implemented by types that can render themselves as SQL expressions.
// Used by the query builder for WHERE conditions, field references, and values.
type EscapeValueable interface {
	EscapeValue() string
}

type escapeValueCtxable interface {
	escapeValueCtx(ctx *renderContext) string
}

// SortValueable is a kind of value that can be used for sorting
type SortValueable interface {
	sortEscapeValue() string
}

// EscapeTableable is a type of value that can be used as a table
type EscapeTableable interface {
	EscapeTable() string
}

// QueryBuilder constructs SQL queries using a fluent API. Create one with [B], then
// chain methods to build SELECT, INSERT, UPDATE, DELETE, or REPLACE queries.
// Execute with [QueryBuilder.RunQuery], [QueryBuilder.ExecQuery], or [QueryBuilder.Render].
//
//	rows, err := psql.B().Select("id", "name").From("users").
//	    Where(map[string]any{"active": true}).
//	    OrderBy(psql.S("name", "ASC")).
//	    Limit(10).
//	    RunQuery(ctx)
type QueryBuilder struct {
	Query       string
	Fields      []any
	Tables      []EscapeTableable
	FieldsSet   []any
	WhereData   WhereAND
	GroupBy     []any
	HavingData  WhereAND
	OrderByData []SortValueable
	LimitData   []int
	renderData  []any // values?

	// conflict/upsert
	ConflictColumns []string // ON CONFLICT (columns)
	ConflictUpdate  []any    // DO UPDATE SET fields (map[string]any entries)
	ConflictNothing bool     // DO NOTHING / INSERT IGNORE

	// flags
	Distinct      bool
	CalcFoundRows bool
	UpdateIgnore  bool
	InsertIgnore  bool
	ForUpdate     bool
	SkipLocked    bool
	NoWait        bool

	err error
}

// B creates a new empty [QueryBuilder]. Chain methods to build a query:
//
//	psql.B().Select("*").From("users").Where(...)
//	psql.B().Update("users").Set(...).Where(...)
//	psql.B().Delete().From("users").Where(...)
func B() *QueryBuilder {
	return new(QueryBuilder)
}

// Select sets the query type to SELECT and specifies the fields to retrieve.
// String arguments are treated as field names. Pass no arguments to select all (*).
func (q *QueryBuilder) Select(fields ...any) *QueryBuilder {
	q.Query = "SELECT"
	if len(fields) > 0 {
		q.Fields = make([]any, 0, len(fields))
		for _, field := range fields {
			switch v := field.(type) {
			case string:
				// consider it to be a field name by default
				q.Fields = append(q.Fields, fieldName(v))
			case any:
				q.Fields = append(q.Fields, v)
			default:
				q.errorf("Unsupported field type %T for select", v)
				return q
			}
		}
	}
	return q
}

func (q *QueryBuilder) errorf(msg string, arg ...any) {
	q.err = fmt.Errorf(msg, arg...)
}

// AlsoSelect adds additional fields to an existing SELECT query.
func (q *QueryBuilder) AlsoSelect(fields ...any) *QueryBuilder {
	if q.Query != "SELECT" {
		q.err = errors.New("invalid QueryBuilder operation")
	}
	q.Fields = append(q.Fields, fields...)
	return q
}

// Update sets the query type to UPDATE and specifies the target table.
// Use [QueryBuilder.Set] to specify the fields to update.
func (q *QueryBuilder) Update(table any) *QueryBuilder {
	q.Query = "UPDATE"
	return q.Table(table)
}

// Replace sets the query type to REPLACE (MySQL) or equivalent upsert.
func (q *QueryBuilder) Replace(table EscapeTableable) *QueryBuilder {
	q.Query = "REPLACE"
	q.Tables = append(q.Tables, table)
	return q
}

// Delete sets the query type to DELETE. Use [QueryBuilder.From] to specify the table.
func (q *QueryBuilder) Delete() *QueryBuilder {
	q.Query = "DELETE"
	return q
}

// Insert sets the query type to INSERT and specifies the fields/values to insert.
func (q *QueryBuilder) Insert(fields ...any) *QueryBuilder {
	q.Query = "INSERT"
	q.FieldsSet = append(q.FieldsSet, fields...)
	return q
}

// Into specifies the target table for INSERT queries.
func (q *QueryBuilder) Into(table EscapeTableable) *QueryBuilder {
	q.Tables = append(q.Tables, table)
	return q
}

// From specifies the source table. Accepts a string (table name) or [EscapeTableable].
func (q *QueryBuilder) From(table any) *QueryBuilder {
	return q.Table(table)
}

// Table adds a table to the query. Accepts a string or [EscapeTableable].
func (q *QueryBuilder) Table(table any) *QueryBuilder {
	switch v := table.(type) {
	case EscapeTableable:
		q.Tables = append(q.Tables, v)
	case string:
		q.Tables = append(q.Tables, tableName(v))
	default:
		q.errorf("unsupported type %T passed as table", v)
	}
	return q
}

// Limit sets the LIMIT clause. With one argument, limits the row count.
// With two arguments, Limit(count, offset) renders as LIMIT count OFFSET offset
// (PostgreSQL/SQLite) or LIMIT count, offset (MySQL).
func (q *QueryBuilder) Limit(v ...int) *QueryBuilder {
	switch len(v) {
	case 0:
		q.LimitData = nil
		return q
	case 1, 2:
		q.LimitData = v
		return q
	default:
		panic("invalid arguments for limit")
	}
}

// Set specifies the fields to update in UPDATE or INSERT queries.
// Typically pass a map[string]any: Set(map[string]any{"name": "Alice"}).
func (q *QueryBuilder) Set(fields ...any) *QueryBuilder {
	q.FieldsSet = append(q.FieldsSet, fields...)
	return q
}

// Where adds conditions to the WHERE clause. Accepts map[string]any for equality
// conditions, [EscapeValueable] for comparisons (e.g., [Equal], [Gt], [Like]),
// or multiple arguments which are joined with AND.
func (q *QueryBuilder) Where(where ...any) *QueryBuilder {
	q.WhereData = append(q.WhereData, where...)
	return q
}

// OrderBy adds ORDER BY clauses. Use [S] to create sort fields:
// OrderBy(psql.S("name", "ASC"), psql.S("created_at", "DESC"))
func (q *QueryBuilder) OrderBy(field ...SortValueable) *QueryBuilder {
	q.OrderByData = append(q.OrderByData, field...)
	return q
}

// GroupByFields adds GROUP BY clause to the query.
func (q *QueryBuilder) GroupByFields(fields ...any) *QueryBuilder {
	for _, field := range fields {
		switch v := field.(type) {
		case string:
			q.GroupBy = append(q.GroupBy, fieldName(v))
		default:
			q.GroupBy = append(q.GroupBy, v)
		}
	}
	return q
}

// Having adds a HAVING clause to the query (used with GROUP BY).
func (q *QueryBuilder) Having(having ...any) *QueryBuilder {
	q.HavingData = append(q.HavingData, having...)
	return q
}

// SetDistinct enables the DISTINCT keyword in the query.
func (q *QueryBuilder) SetDistinct() *QueryBuilder {
	q.Distinct = true
	return q
}

// OnConflict specifies the conflict columns for INSERT ... ON CONFLICT.
func (q *QueryBuilder) OnConflict(columns ...string) *QueryBuilder {
	q.ConflictColumns = columns
	return q
}

// DoUpdate specifies the fields to update on conflict. Accepts map[string]any
// entries, similar to [QueryBuilder.Set].
func (q *QueryBuilder) DoUpdate(fields ...any) *QueryBuilder {
	q.ConflictUpdate = append(q.ConflictUpdate, fields...)
	return q
}

// DoNothing sets the ON CONFLICT action to DO NOTHING (PostgreSQL/SQLite)
// or INSERT IGNORE (MySQL).
func (q *QueryBuilder) DoNothing() *QueryBuilder {
	q.ConflictNothing = true
	return q
}

// SetForUpdate adds FOR UPDATE locking to the query.
func (q *QueryBuilder) SetForUpdate() *QueryBuilder {
	q.ForUpdate = true
	return q
}

// SetSkipLocked adds SKIP LOCKED after FOR UPDATE. Rows locked by other
// transactions are skipped instead of blocking.
func (q *QueryBuilder) SetSkipLocked() *QueryBuilder {
	q.ForUpdate = true
	q.SkipLocked = true
	return q
}

// SetNoWait adds NOWAIT after FOR UPDATE. The query fails immediately if
// any selected row is locked by another transaction.
func (q *QueryBuilder) SetNoWait() *QueryBuilder {
	q.ForUpdate = true
	q.NoWait = true
	return q
}

// Join adds a JOIN clause to the query.
func (q *QueryBuilder) Join(joinType, table string, condition ...any) *QueryBuilder {
	q.renderData = append(q.renderData, &joinClause{
		joinType:  joinType,
		table:     tableName(table),
		condition: condition,
	})
	return q
}

// LeftJoin adds a LEFT JOIN clause to the query.
func (q *QueryBuilder) LeftJoin(table string, condition ...any) *QueryBuilder {
	return q.Join("LEFT", table, condition...)
}

// InnerJoin adds an INNER JOIN clause to the query.
func (q *QueryBuilder) InnerJoin(table string, condition ...any) *QueryBuilder {
	return q.Join("INNER", table, condition...)
}

// RightJoin adds a RIGHT JOIN clause to the query.
func (q *QueryBuilder) RightJoin(table string, condition ...any) *QueryBuilder {
	return q.Join("RIGHT", table, condition...)
}

type joinClause struct {
	joinType  string
	table     tableName
	condition []any
}

// SubIn wraps a [QueryBuilder] subquery for use with IN (subquery) in WHERE conditions:
//
//	psql.B().Select().From("users").Where(map[string]any{
//	    "id": &psql.SubIn{psql.B().Select("user_id").From("orders")},
//	})
//	// → ... WHERE "id" IN (SELECT "user_id" FROM "orders")
type SubIn struct {
	Sub *QueryBuilder
}

// escapeValueCtx renders the QueryBuilder as a parenthesized subquery, sharing
// the parent context's args slice so parameter numbering continues correctly.
func (q *QueryBuilder) escapeValueCtx(ctx *renderContext) string {
	savedReq := ctx.req
	err := q.render(ctx)
	if err != nil {
		ctx.req = savedReq
		return "NULL"
	}
	subSQL := strings.Join(ctx.req, " ")
	ctx.req = savedReq
	return "(" + subSQL + ")"
}

// EscapeValue renders the QueryBuilder as a parenthesized subquery (non-parameterized).
func (q *QueryBuilder) EscapeValue() string {
	return q.escapeValueCtx(nil)
}

// Apply runs the given scopes on this query builder, returning the modified builder.
// Scopes can add WHERE, ORDER BY, LIMIT, or any other clause.
func (q *QueryBuilder) Apply(scopes ...Scope) *QueryBuilder {
	for _, s := range scopes {
		q = s(q)
	}
	return q
}

// Render generates the SQL query string for the current engine. Values are
// embedded directly (not parameterized). For parameterized queries, use [QueryBuilder.RenderArgs].
func (q *QueryBuilder) Render(ctx context.Context) (string, error) {
	// Generate the actual SQL query
	e := GetBackend(ctx).Engine()
	rctx := &renderContext{e: e, d: e.dialect(), useArgs: false}
	err := q.render(rctx)
	if err != nil {
		return "", err
	}
	return strings.Join(rctx.req, " "), nil
}

// RenderArgs generates the SQL query string with parameterized placeholders and
// returns the arguments separately. Uses $1/$2/... for PostgreSQL and ? for MySQL/SQLite.
func (q *QueryBuilder) RenderArgs(ctx context.Context) (string, []any, error) {
	// Generate the actual SQL query
	e := GetBackend(ctx).Engine()
	rctx := &renderContext{e: e, d: e.dialect(), useArgs: true}
	err := q.render(rctx)
	if err != nil {
		return "", nil, err
	}
	return strings.Join(rctx.req, " "), rctx.args, nil
}

// RunQuery executes the query and returns *sql.Rows for reading results.
// The caller must close the returned rows. Typically used for SELECT queries.
func (q *QueryBuilder) RunQuery(ctx context.Context) (*sql.Rows, error) {
	query, args, err := q.RenderArgs(ctx)
	if err != nil {
		return nil, err
	}

	res, err := doQueryContext(ctx, query, args...)
	if err != nil {
		return nil, &Error{query, err}
	}
	return res, nil
}

// ExecQuery executes the query and returns sql.Result. Used for INSERT, UPDATE,
// DELETE, and other non-row-returning queries.
func (q *QueryBuilder) ExecQuery(ctx context.Context) (sql.Result, error) {
	query, args, err := q.RenderArgs(ctx)
	if err != nil {
		return nil, err
	}
	return ExecContext(ctx, query, args...)
}

// Prepare creates a prepared statement from the query. The caller must close
// the returned statement.
func (q *QueryBuilder) Prepare(ctx context.Context) (*sql.Stmt, error) {
	query, _, err := q.RenderArgs(ctx)
	if err != nil {
		return nil, err
	}

	return doPrepareContext(ctx, query)
}

func (q *QueryBuilder) render(ctx *renderContext) error {
	if q.err != nil {
		return q.err
	}

	// Generate the actual SQL query
	ctx.req = []string{q.Query}
	var err error

	switch q.Query {
	case "SELECT":
		if q.Distinct {
			ctx.append("DISTINCT")
		}
		if q.CalcFoundRows {
			ctx.append("SQL_CALC_FOUND_ROWS")
		}
		err = q.renderFields(ctx)
		if err != nil {
			return err
		}
		ctx.append("FROM")
		err = q.renderTables(ctx)
		if err != nil {
			return err
		}
	case "DELETE":
		ctx.append("FROM")
		err = q.renderTables(ctx)
		if err != nil {
			return err
		}
	case "UPDATE":
		if q.UpdateIgnore {
			ctx.append("IGNORE")
		}
		fallthrough
	case "REPLACE":
		err = q.renderTables(ctx)
		if err != nil {
			return err
		}
		ctx.append("SET")
		ctx.append(escapeWhere(ctx, q.FieldsSet, ","))
	case "INSERT":
		switch ctx.e {
		case EnginePostgreSQL:
			// PostgreSQL: use (cols) VALUES (vals) format
			ctx.append("INTO")
			err = q.renderTables(ctx)
			if err != nil {
				return err
			}
			colsVals := q.renderInsertColsVals(ctx)
			ctx.append(colsVals)
			// ON CONFLICT clause
			if len(q.ConflictUpdate) > 0 && len(q.ConflictColumns) > 0 {
				conflictCols := make([]string, len(q.ConflictColumns))
				for i, c := range q.ConflictColumns {
					conflictCols[i] = QuoteName(c)
				}
				ctx.append("ON CONFLICT (" + strings.Join(conflictCols, ",") + ") DO UPDATE SET")
				ctx.append(escapeWhere(ctx, q.ConflictUpdate, ","))
			} else if q.InsertIgnore || q.ConflictNothing {
				ctx.append("ON CONFLICT DO NOTHING")
			}
		case EngineSQLite:
			// SQLite: use (cols) VALUES (vals) format
			if q.InsertIgnore || q.ConflictNothing {
				ctx.req = []string{"INSERT", "OR", "IGNORE"}
			}
			ctx.append("INTO")
			err = q.renderTables(ctx)
			if err != nil {
				return err
			}
			colsVals := q.renderInsertColsVals(ctx)
			ctx.append(colsVals)
			if len(q.ConflictUpdate) > 0 && len(q.ConflictColumns) > 0 {
				conflictCols := make([]string, len(q.ConflictColumns))
				for i, c := range q.ConflictColumns {
					conflictCols[i] = QuoteName(c)
				}
				ctx.append("ON CONFLICT (" + strings.Join(conflictCols, ",") + ") DO UPDATE SET")
				ctx.append(escapeWhere(ctx, q.ConflictUpdate, ","))
			}
		default:
			// MySQL / Unknown: use SET syntax (MySQL-native)
			if q.InsertIgnore || q.ConflictNothing {
				ctx.append("IGNORE")
			}
			ctx.append("INTO")
			err = q.renderTables(ctx)
			if err != nil {
				return err
			}
			ctx.append("SET")
			ctx.append(escapeWhere(ctx, q.FieldsSet, ","))
			if len(q.ConflictUpdate) > 0 {
				ctx.append("ON DUPLICATE KEY UPDATE")
				ctx.append(escapeWhere(ctx, q.ConflictUpdate, ","))
			}
		}
	case "INSERT_SELECT":
		if len(q.Tables) < 2 {
			return fmt.Errorf("INSERT SELECT requires at least two tables")
		}
		ctx.req = []string{"INSERT"}
		if q.InsertIgnore {
			ctx.append("IGNORE")
		}
		table := q.Tables[0]
		ctx.append("INTO", table.EscapeTable())
		ctx.append("SELECT")
		if q.Distinct {
			ctx.append("DISTINCT")
		}
		err = q.renderFields(ctx)
		if err != nil {
			return err
		}
		ctx.append("FROM")
		err = q.renderTables(ctx)
		if err != nil {
			return err
		}
	}

	if len(q.WhereData) > 0 {
		ctx.append("WHERE", q.WhereData.escapeValueCtx(ctx))
	}
	if len(q.GroupBy) > 0 {
		ctx.append("GROUP BY")
		err = ctx.appendCommaValues(q.GroupBy...)
		if err != nil {
			return err
		}
	}
	if len(q.HavingData) > 0 {
		ctx.append("HAVING", q.HavingData.escapeValueCtx(ctx))
	}
	if len(q.OrderByData) > 0 {
		ctx.append("ORDER BY")
		err = ctx.appendCommaValuesSort(q.OrderByData...)
		if err != nil {
			return err
		}
	}
	switch len(q.LimitData) {
	case 1:
		ctx.append("LIMIT", strconv.Itoa(q.LimitData[0]))
	case 2:
		ctx.append(ctx.d.LimitOffset(q.LimitData[0], q.LimitData[1]))
	}
	if q.ForUpdate {
		ctx.append("FOR UPDATE")
		if q.SkipLocked {
			ctx.append("SKIP LOCKED")
		} else if q.NoWait {
			ctx.append("NOWAIT")
		}
	}

	return nil
}

func (q *QueryBuilder) renderFields(ctx *renderContext) error {
	if len(q.Fields) == 0 {
		ctx.append("*")
		return nil
	}
	return ctx.appendCommaValues(q.Fields...)
}

// renderInsertColsVals renders FieldsSet as "(col1,col2) VALUES (val1,val2)" format.
// It iterates the FieldsSet entries (expected to be map[string]any) and extracts
// sorted columns and their corresponding values.
func (q *QueryBuilder) renderInsertColsVals(ctx *renderContext) string {
	var cols []string
	var vals []string

	for _, fs := range q.FieldsSet {
		switch m := fs.(type) {
		case map[string]any:
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				cols = append(cols, QuoteName(k))
				vals = append(vals, escapeCtx(ctx, m[k]))
			}
		}
	}

	if len(cols) == 0 {
		// Fallback to SET syntax if no map entries found
		return "SET " + escapeWhere(ctx, q.FieldsSet, ",")
	}

	return "(" + strings.Join(cols, ",") + ") VALUES (" + strings.Join(vals, ",") + ")"
}

func (q *QueryBuilder) renderTables(ctx *renderContext) error {
	b := &strings.Builder{}

	for n, v := range q.Tables {
		if n != 0 {
			b.WriteByte(',')
		}
		b.WriteString(v.EscapeTable())
	}

	// Append JOIN clauses
	for _, rd := range q.renderData {
		if j, ok := rd.(*joinClause); ok {
			b.WriteByte(' ')
			b.WriteString(j.joinType)
			b.WriteString(" JOIN ")
			b.WriteString(j.table.EscapeTable())
			if len(j.condition) > 0 {
				b.WriteString(" ON ")
				b.WriteString(escapeWhere(ctx, j.condition, " AND "))
			}
		}
	}

	ctx.append(b.String())
	return nil
}
