package psql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	// flags
	Distinct      bool
	CalcFoundRows bool
	UpdateIgnore  bool
	InsertIgnore  bool
	ForUpdate     bool

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

// Render generates the SQL query string for the current engine. Values are
// embedded directly (not parameterized). For parameterized queries, use [QueryBuilder.RenderArgs].
func (q *QueryBuilder) Render(ctx context.Context) (string, error) {
	// Generate the actual SQL query
	rctx := &renderContext{e: GetBackend(ctx).Engine(), useArgs: false}
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
	rctx := &renderContext{e: GetBackend(ctx).Engine(), useArgs: true}
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
		if q.InsertIgnore {
			ctx.append("IGNORE")
		}
		ctx.append("INTO")
		err = q.renderTables(ctx)
		if err != nil {
			return err
		}
		ctx.append("SET")
		ctx.append(escapeWhere(ctx, q.FieldsSet, ","))
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
		// PostgreSQL and SQLite use LIMIT x OFFSET y, MySQL uses LIMIT x, y
		if ctx.e == EnginePostgreSQL || ctx.e == EngineSQLite {
			ctx.append("LIMIT", strconv.Itoa(q.LimitData[0]))
			ctx.append("OFFSET", strconv.Itoa(q.LimitData[1]))
		} else {
			ctx.append("LIMIT", strconv.Itoa(q.LimitData[0])+",", strconv.Itoa(q.LimitData[1]))
		}
	}
	if q.ForUpdate {
		ctx.append("FOR UPDATE")
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
