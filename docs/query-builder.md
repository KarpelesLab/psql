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
&psql.ILike{psql.F("name"), "john%"}      // ILIKE on PostgreSQL, LIKE COLLATE NOCASE on SQLite
```

### Multiple Conditions

```go
// Multiple arguments are joined with AND
query := psql.B().Select().From("users").Where(
    psql.Equal(psql.F("status"), "active"),
    psql.Gte(psql.F("age"), 18),
)
```

### Subqueries

Use `SubIn` for IN (subquery) conditions:

```go
// WHERE "id" IN (SELECT "user_id" FROM "orders")
query := psql.B().Select().From("users").Where(map[string]any{
    "id": &psql.SubIn{Sub: psql.B().Select("user_id").From("orders")},
})

// NOT IN subquery
query := psql.B().Select().From("users").Where(map[string]any{
    "id": &psql.Not{V: &psql.SubIn{Sub: psql.B().Select("user_id").From("banned")}},
})
```

### Any (PostgreSQL-Optimized Array Comparison)

On PostgreSQL with parameterized queries, `Any` uses `= ANY($N)` passing the slice as a single array parameter. On MySQL/SQLite it expands to `IN(...)`:

```go
query := psql.B().Select().From("users").Where(map[string]any{
    "id": &psql.Any{Values: []int64{1, 2, 3}},
})
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

## FOR UPDATE

```go
query := psql.B().Select().From("users").Where(...).SetForUpdate()
// SELECT ... FOR UPDATE

query := psql.B().Select().From("users").Where(...).SetSkipLocked()
// SELECT ... FOR UPDATE SKIP LOCKED

query := psql.B().Select().From("users").Where(...).SetNoWait()
// SELECT ... FOR UPDATE NOWAIT
```

FOR UPDATE is silently omitted on SQLite (which uses file/WAL-level locking).

## ON CONFLICT (Upsert)

```go
// INSERT ... ON CONFLICT DO NOTHING
query := psql.B().Insert().Into("users").
    Set(map[string]any{"id": 1, "name": "Alice"}).
    DoNothing()

// INSERT ... ON CONFLICT (id) DO UPDATE SET name=...
query := psql.B().Insert().Into("users").
    Set(map[string]any{"id": 1, "name": "Alice"}).
    OnConflict("id").
    DoUpdate(map[string]any{"name": "Alice"})
```

## SET Expressions (UPDATE)

### Increment / Decrement

Atomically increment or decrement a field:

```go
psql.B().Update("counters").
    Set(map[string]any{"views": psql.Incr(1)}).
    Where(map[string]any{"id": 42})
// UPDATE "counters" SET "views"="views"+1 WHERE "id"=42

psql.B().Update("inventory").
    Set(map[string]any{"stock": psql.Decr(1)}).
    Where(map[string]any{"id": 42})
// UPDATE "inventory" SET "stock"="stock"-1 WHERE "id"=42
```

### SetRaw

Use raw SQL in a SET clause:

```go
psql.B().Update("users").
    Set(map[string]any{"last_seen": &psql.SetRaw{SQL: "NOW()"}}).
    Where(map[string]any{"id": 42})
```

## Scopes

Apply reusable query modifiers:

```go
var Active psql.Scope = func(q *psql.QueryBuilder) *psql.QueryBuilder {
    return q.Where(map[string]any{"Status": "active"})
}

query := psql.B().Select().From("users").Apply(Active)
```

See [Scopes & Lazy](scopes-lazy.md) for details.

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

### Typed Query Execution

Scan query results directly into typed structs:

```go
// Scan all rows into []*User
users, err := psql.RunQueryT[User](ctx, query)

// Scan a single row into *User (returns os.ErrNotExist if no rows)
user, err := psql.RunQueryTOne[User](ctx, query)
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
        "login_count": psql.Incr(1),
    }).
    Where(map[string]any{"id": userID})

// Complex search with case-insensitive matching
search := psql.B().
    Select().
    From("products").
    Where(map[string]any{
        "status":      "available",
        "name":        &psql.ILike{psql.F("name"), "%" + searchTerm + "%"},
        "category_id": psql.WhereOR{1, 2, 3},
    }).
    OrderBy(psql.S("price", "ASC"))
```
