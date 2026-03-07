# Query Builder

The query builder provides a fluent interface for constructing SQL queries programmatically. It supports SELECT, INSERT, UPDATE, DELETE, and REPLACE operations with full support for WHERE clauses, JOINs, ORDER BY, LIMIT, and more.

## Basic Usage

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

## Helper Functions

- `psql.F("field")` - Field reference (column name, properly quoted)
- `psql.V("value")` - Value literal
- `psql.S("field", "ASC")` - Sort field with direction
- `psql.Raw("SQL")` - Raw SQL (use carefully)

## WHERE Conditions

### Map Syntax

The simplest way to express conditions:

```go
// Equality
query := psql.B().Select().From("users").Where(map[string]any{"id": 123})

// IS NULL
query := psql.B().Select().From("users").Where(map[string]any{"deleted_at": nil})

// IN (pass a slice)
query := psql.B().Select().From("users").Where(map[string]any{"id": []int{1, 2, 3}})

// OR conditions for same field
query := psql.B().Select().From("users").Where(map[string]any{
    "status": psql.WhereOR{"active", "pending"},
})
```

### Comparison Operators

```go
psql.Equal(psql.F("status"), "active")    // status = 'active'
psql.Lt(psql.F("age"), 18)                // age < 18
psql.Lte(psql.F("age"), 65)               // age <= 65
psql.Gt(psql.F("age"), 18)                // age > 18
psql.Gte(psql.F("age"), 18)               // age >= 18
psql.Between(psql.F("age"), 18, 65)       // age BETWEEN 18 AND 65
&psql.Not{V: value}                        // negation (!=, IS NOT NULL, NOT LIKE)
&psql.Like{psql.F("name"), "John%"}       // name LIKE 'John%'
```

### Multiple Conditions

```go
// Multiple arguments are joined with AND
query := psql.B().Select().From("users").Where(
    psql.Equal(psql.F("status"), "active"),
    psql.Gte(psql.F("age"), 18),
)
```

## ORDER BY and LIMIT

```go
query := psql.B().Select().From("users").
    OrderBy(psql.S("created_at", "DESC"), psql.S("name", "ASC")).
    Limit(10)

// With offset
query := psql.B().Select().From("users").
    OrderBy(psql.S("created_at", "DESC")).
    Limit(10, 20)
// MySQL:      LIMIT 10, 20
// PostgreSQL: LIMIT 10 OFFSET 20
```

## JOINs

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

## GROUP BY and HAVING

```go
query := psql.B().
    Select("status", psql.Raw("COUNT(*)")).
    From("users").
    GroupByFields("status")

query := psql.B().
    Select("status", psql.Raw("COUNT(*) as cnt")).
    From("users").
    GroupByFields("status").
    Having(psql.Gt(psql.Raw("COUNT(*)"), 5))
```

## DISTINCT

```go
query := psql.B().Select("name").From("users").SetDistinct()
// SELECT DISTINCT "name" FROM "users"
```

## Raw SQL

For queries that don't fit the builder, use `psql.Q()`:

```go
err := psql.Q(`DROP TABLE IF EXISTS "old_table"`).Exec(ctx)
```

## Executing Queries

```go
// Get the SQL string
sql, err := query.Render(ctx)

// Get SQL with placeholders and arguments
sql, args, err := query.RenderArgs(ctx)

// Execute SELECT (returns rows)
rows, err := query.RunQuery(ctx)
defer rows.Close()

// Execute INSERT/UPDATE/DELETE (returns result)
result, err := query.ExecQuery(ctx)

// Prepare a statement
stmt, err := query.Prepare(ctx)
defer stmt.Close()
```

## Complete Examples

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

// Update user's last login
update := psql.B().
    Update("users").
    Set(map[string]any{
        "last_login":  time.Now(),
        "login_count": psql.Raw("login_count + 1"),
    }).
    Where(map[string]any{"id": userID})

// Complex search
search := psql.B().
    Select().
    From("products").
    Where(map[string]any{
        "status":      "available",
        "name":        &psql.Like{psql.F("name"), "%" + searchTerm + "%"},
        "category_id": psql.WhereOR{1, 2, 3},
    }).
    OrderBy(psql.S("price", "ASC"))
```
