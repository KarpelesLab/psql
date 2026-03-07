[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/psql.svg)](https://pkg.go.dev/github.com/KarpelesLab/psql)
[![Build Status](https://github.com/KarpelesLab/psql/actions/workflows/test.yml/badge.svg)](https://github.com/KarpelesLab/psql/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/KarpelesLab/psql/badge.svg?branch=master)](https://coveralls.io/github/KarpelesLab/psql?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/KarpelesLab/psql)](https://goreportcard.com/report/github.com/KarpelesLab/psql)

# psql

Platform SQL code, including object load/save & query builder.

This works in some ways similar to `gorm` but with focus on supporting and using modern Go syntax & features.

## Object binding

After defining a structure, you can use it to load/save data from database.

```go
type Table1 struct {
    Key uint64 `sql:",key=PRIMARY"`
    Name string `sql:"Name,type=VARCHAR,size=64"`
}

// ...

obj, err := psql.Get[Table1](context.Background(), map[string]any{"Key": 42}) // this fetches entry with Key=42
```

### Enum Support

The library supports SQL ENUM types with different implementations for MySQL and PostgreSQL:

```go
type StatusEnum string

const (
    StatusPending  StatusEnum = "pending"
    StatusActive   StatusEnum = "active"
    StatusInactive StatusEnum = "inactive"
)

type MyTable struct {
    ID     uint64     `sql:",key=PRIMARY"`
    Status StatusEnum `sql:",type=enum,values=pending,active,inactive"`
}
```

In MySQL, this creates a standard ENUM column.

In PostgreSQL, this creates a custom type named according to the `EnumTypeName` method of the configured Namer (default: `enum_tablename_columnname`) and automatically handles type creation and column mapping.

## go 1.23

New go 1.23 iterators can be used

```go
res, err := psql.Iter[Table1](context.Background(), map[string]any{"Type": "A"}) // this fetches entries with Type=A
if err != nil {
    return err
}
for v := range res {
    // v is of type *Table1
}
```

## Query builder

The query builder provides a fluent interface for constructing SQL queries programmatically. It supports SELECT, INSERT, UPDATE, DELETE, and REPLACE operations with full support for WHERE clauses, JOINs, ORDER BY, LIMIT, and more.

### Basic Usage

Start building a query with `psql.B()`:

```go
// Simple SELECT
query := psql.B().Select("name", "email").From("users")

// SELECT with WHERE clause
query := psql.B().Select().From("users").Where(map[string]any{"status": "active"})

// UPDATE
query := psql.B().Update("users").Set(map[string]any{"status": "inactive"}).Where(map[string]any{"id": 123})

// DELETE
query := psql.B().Delete().From("users").Where(map[string]any{"id": 123})
```

### Helper Functions

The library provides helper functions for working with SQL identifiers and values:

- `psql.F("field")` - Field reference (column name)
- `psql.V("value")` - Value literal
- `psql.S("field", "ASC")` - Sort field with direction
- `psql.Raw("SQL")` - Raw SQL injection (use carefully)

### WHERE Conditions

The query builder supports various WHERE condition operators:

```go
// Equality
query := psql.B().Select().From("users").Where(map[string]any{"id": 123})

// Using comparison operators
query := psql.B().Select().From("users").Where(psql.Gte(psql.F("age"), 18))

// LIKE operator
query := psql.B().Select().From("users").Where(&psql.Like{psql.F("name"), "John%"})

// IS NULL / IS NOT NULL
query := psql.B().Select().From("users").Where(map[string]any{"deleted_at": nil})

// Complex conditions with OR
query := psql.B().Select().From("users").Where(map[string]any{
    "status": psql.WhereOR{"active", "pending"},
})

// Multiple conditions (AND)
query := psql.B().Select().From("users").Where(
    psql.Equal(psql.F("status"), "active"),
    psql.Gte(psql.F("age"), 18),
)
```

### Comparison Operators

Available comparison functions:
- `psql.Equal(field, value)` - Equality (=)
- `psql.Lt(field, value)` - Less than (<)
- `psql.Lte(field, value)` - Less than or equal (<=)
- `psql.Gt(field, value)` - Greater than (>)
- `psql.Gte(field, value)` - Greater than or equal (>=)
- `psql.Between(field, start, end)` - BETWEEN (inclusive range)
- `&psql.Not{V: value}` - Negate a condition (IS NOT NULL, !=, NOT LIKE, etc.)
- `psql.WhereOR{value1, value2, ...}` - OR conditions for the same field

### Advanced Features

#### ORDER BY and LIMIT

```go
query := psql.B().Select().From("users").
    OrderBy(psql.S("created_at", "DESC"), psql.S("name", "ASC")).
    Limit(10, 20) // limit 10, offset 20
// Renders as "LIMIT 10, 20" for MySQL
// Renders as "LIMIT 10 OFFSET 20" for PostgreSQL
```

#### Raw SQL

For complex queries, you can inject raw SQL:

```go
query := psql.B().Select(psql.Raw("COUNT(DISTINCT user_id)")).From("orders")
```

#### DISTINCT

```go
query := psql.B().Select("name").From("users").SetDistinct()
// SELECT DISTINCT "name" FROM "users"
```

#### JOINs

```go
// INNER JOIN
query := psql.B().
    Select(psql.F("users", "name"), psql.F("orders", "total")).
    From("users").
    InnerJoin("orders", psql.Equal(psql.F("users.id"), psql.F("orders.user_id")))

// LEFT JOIN
query := psql.B().
    Select().From("users").
    LeftJoin("profiles", psql.Equal(psql.F("users.id"), psql.F("profiles.user_id")))

// RIGHT JOIN
query := psql.B().
    Select().From("users").
    RightJoin("orders", psql.Equal(psql.F("users.id"), psql.F("orders.user_id")))
```

#### GROUP BY and HAVING

```go
// GROUP BY
query := psql.B().
    Select("status", psql.Raw("COUNT(*)")).
    From("users").
    GroupByFields("status")

// GROUP BY with HAVING
query := psql.B().
    Select("status", psql.Raw("COUNT(*) as cnt")).
    From("users").
    GroupByFields("status").
    Having(psql.Gt(psql.Raw("COUNT(*)"), 5))
```

#### Aggregate Functions

```go
// COUNT
query := psql.B().Select(psql.Raw("COUNT(*)")).From("users")
```

### Executing Queries

The query builder provides several methods to execute the built query:

```go
// Get the SQL string
sql, err := query.Render(ctx)

// Get SQL with placeholders and arguments (for prepared statements)
sql, args, err := query.RenderArgs(ctx)

// Execute a query that returns rows (SELECT)
rows, err := query.RunQuery(ctx)
defer rows.Close()

// Execute a query that doesn't return rows (INSERT, UPDATE, DELETE)
result, err := query.ExecQuery(ctx)

// Prepare a statement for repeated execution
stmt, err := query.Prepare(ctx)
defer stmt.Close()
```

### Complete Examples

```go
// Find active users older than 18, ordered by name
users := psql.B().
    Select("id", "name", "email").
    From("users").
    Where(
        psql.Equal(psql.F("status"), "active"),
        psql.Gt(psql.F("age"), 18),
    ).
    OrderBy(psql.S("name", "ASC")).
    Limit(50)

// Update user's last login time
update := psql.B().
    Update("users").
    Set(map[string]any{
        "last_login": time.Now(),
        "login_count": psql.Raw("login_count + 1"),
    }).
    Where(map[string]any{"id": userID})

// Complex search with LIKE and OR conditions
search := psql.B().
    Select().
    From("products").
    Where(map[string]any{
        "status": "available",
        "name": &psql.Like{psql.F("name"), "%" + searchTerm + "%"},
        "category_id": psql.WhereOR{1, 2, 3},
    }).
    OrderBy(psql.S("price", "ASC"))
```
