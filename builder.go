package psql

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type EscapeValueable interface {
	EscapeValue() string
}

// EscapeTableable is a type of value that can be used as a table
type EscapeTableable interface {
	EscapeTable() string
}

type QueryBuilder struct {
	Query      string
	Fields     []any
	Tables     []EscapeTableable
	FieldsSet  []any
	WhereData  WhereAND
	GroupBy    []any
	OrderBy    []any
	Limit      []int
	renderData []any // values?

	// flags
	Distinct      bool
	CalcFoundRows bool
	UpdateIgnore  bool
	InsertIgnore  bool
	ForUpdate     bool

	err error
}

func B() *QueryBuilder {
	return new(QueryBuilder)
}

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

func (q *QueryBuilder) AlsoSelect(fields ...any) *QueryBuilder {
	if q.Query != "SELECT" {
		q.err = errors.New("Invalid QueryBuilder operation")
	}
	q.Fields = append(q.Fields, fields...)
	return q
}

func (q *QueryBuilder) Update(table any) *QueryBuilder {
	q.Query = "UPDATE"
	return q.Table(table)
}

func (q *QueryBuilder) Replace(table EscapeTableable) *QueryBuilder {
	q.Query = "REPLACE"
	q.Tables = append(q.Tables, table)
	return q
}

func (q *QueryBuilder) Delete() *QueryBuilder {
	q.Query = "DELETE"
	return q
}

func (q *QueryBuilder) Insert(fields ...any) *QueryBuilder {
	q.Query = "INSERT"
	q.FieldsSet = append(q.FieldsSet, fields...)
	return q
}

func (q *QueryBuilder) Into(table EscapeTableable) *QueryBuilder {
	q.Tables = append(q.Tables, table)
	return q
}

func (q *QueryBuilder) From(table any) *QueryBuilder {
	return q.Table(table)
}

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

func (q *QueryBuilder) Set(fields ...any) *QueryBuilder {
	q.FieldsSet = append(q.FieldsSet, fields...)
	return q
}

func (q *QueryBuilder) Where(where ...any) *QueryBuilder {
	q.WhereData = append(q.WhereData, where...)
	return q
}

func (q *QueryBuilder) Render() (string, error) {
	if q.err != nil {
		return "", q.err
	}

	// Generate the actual SQL query
	ctx := &renderContext{
		req: []string{q.Query},
	}
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
			return "", err
		}
		ctx.append("FROM")
		err = q.renderTables(ctx)
		if err != nil {
			return "", err
		}
	case "DELETE":
		ctx.append("FROM")
		err = q.renderTables(ctx)
		if err != nil {
			return "", err
		}
	case "UPDATE":
		if q.UpdateIgnore {
			ctx.append("IGNORE")
		}
		fallthrough
	case "REPLACE":
		err = q.renderTables(ctx)
		if err != nil {
			return "", err
		}
		ctx.append("SET")
		err = ctx.appendCommaValues(q.FieldsSet...)
		if err != nil {
			return "", err
		}
	case "INSERT":
		if q.InsertIgnore {
			ctx.append("IGNORE")
		}
		ctx.append("INTO")
		err = q.renderTables(ctx)
		if err != nil {
			return "", err
		}
		ctx.append("SET")
		err = ctx.appendCommaValues(q.FieldsSet...)
		if err != nil {
			return "", err
		}
	case "INSERT_SELECT":
		if len(q.Tables) < 2 {
			return "", fmt.Errorf("INSERT SELECT requires at least two tables")
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
			return "", err
		}
		ctx.append("FROM")
		err = q.renderTables(ctx)
		if err != nil {
			return "", err
		}
	}

	if len(q.WhereData) > 0 {
		ctx.append("WHERE", q.WhereData.EscapeValue())
	}
	if len(q.GroupBy) > 0 {
		ctx.append("GROUP BY")
		err = ctx.appendCommaValues(q.GroupBy...)
		if err != nil {
			return "", err
		}
	}
	if len(q.OrderBy) > 0 {
		ctx.append("ORDER BY")
		err = ctx.appendCommaValues(q.OrderBy...)
		if err != nil {
			return "", err
		}
	}
	switch len(q.Limit) {
	case 1:
		ctx.append("LIMIT", strconv.Itoa(q.Limit[0]))
	case 2:
		ctx.append("LIMIT", strconv.Itoa(q.Limit[0])+",", strconv.Itoa(q.Limit[1]))
	}
	if q.ForUpdate {
		ctx.append("FOR UPDATE")
	}

	return strings.Join(ctx.req, " "), nil
}

func (q *QueryBuilder) renderFields(ctx *renderContext) error {
	if len(q.Fields) == 0 {
		ctx.append("*")
		return nil
	}
	return ctx.appendCommaValues(q.Fields...)
}

func (q *QueryBuilder) renderTables(ctx *renderContext) error {
	// TODO extract joins, etc
	b := &strings.Builder{}

	for n, v := range q.Tables {
		if n != 0 {
			b.WriteByte(',')
		}
		b.WriteString(v.EscapeTable())
	}

	ctx.append(b.String())
	return nil
}
