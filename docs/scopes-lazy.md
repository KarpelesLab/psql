# Scopes & Lazy Loading

## Scopes

Scopes are reusable query modifier functions. Define them once, apply them everywhere.

### Defining Scopes

A `Scope` is a function that takes a `*QueryBuilder` and returns a modified `*QueryBuilder`:

```go
var Active psql.Scope = func(q *psql.QueryBuilder) *psql.QueryBuilder {
    return q.Where(map[string]any{"Status": "active"})
}

var Recent psql.Scope = func(q *psql.QueryBuilder) *psql.QueryBuilder {
    return q.OrderBy(psql.S("CreatedAt", "DESC"))
}

func LimitN(n int) psql.Scope {
    return func(q *psql.QueryBuilder) *psql.QueryBuilder {
        return q.Limit(n)
    }
}
```

### Using Scopes with Fetch

Pass scopes via `WithScope` as fetch options:

```go
// Apply scopes to Fetch
users, err := psql.Fetch[User](ctx, nil, psql.WithScope(Active, Recent, LimitN(10)))

// Combine scopes with other options
users, err := psql.Fetch[User](ctx,
    map[string]any{"Role": "admin"},
    psql.WithScope(Active),
    psql.Sort(psql.S("Name", "ASC")),
)

// Works with Get and Count too
count, err := psql.Count[User](ctx, nil, psql.WithScope(Active))
```

### Using Scopes with QueryBuilder

Apply scopes directly on the query builder:

```go
query := psql.B().Select().From("users").Apply(Active).Apply(Recent)
rows, err := query.RunQuery(ctx)
```

### Composing Scopes

Since scopes are just functions, combine them freely:

```go
func ActiveRecent(n int) psql.Scope {
    return func(q *psql.QueryBuilder) *psql.QueryBuilder {
        return q.
            Where(map[string]any{"Status": "active"}).
            OrderBy(psql.S("CreatedAt", "DESC")).
            Limit(n)
    }
}
```

## Lazy Loading

`Future[T]` provides batch-optimized deferred database queries. Multiple futures for the same table and column are automatically batched into a single `WHERE col IN (...)` query when any one of them is resolved.

### Basic Usage

```go
// Create futures (no database query yet)
future1 := psql.Lazy[User]("ID", "1")
future2 := psql.Lazy[User]("ID", "2")
future3 := psql.Lazy[User]("ID", "3")

// Resolving any future batches all pending futures into one query
user1, err := future1.Resolve(ctx)  // executes: SELECT ... WHERE ID IN (1, 2, 3)
user2, err := future2.Resolve(ctx)  // already resolved by the batch above
user3, err := future3.Resolve(ctx)  // already resolved
```

### How It Works

1. `Lazy[T]("col", "val")` creates a `Future[T]` and registers it in a pending pool
2. When `Resolve(ctx)` is called on any future, it collects all pending futures for the same column
3. A single batch query `SELECT ... WHERE col IN (val1, val2, ...)` is executed
4. Results are distributed to all waiting futures

This is ideal for resolving references across many objects without N+1 queries, especially in API handlers or template rendering.

### Deduplication

Multiple calls to `Lazy[T]("ID", "42")` with the same column and value return the same `Future` instance:

```go
f1 := psql.Lazy[User]("ID", "42")
f2 := psql.Lazy[User]("ID", "42")
// f1 == f2 (same pointer)
```

### Concurrent Safety

Futures are safe for concurrent use. Multiple goroutines can call `Resolve` simultaneously; only one will perform the actual query, and others will wait for the result.

### JSON Marshaling

`Future[T]` implements `json.Marshaler`. Marshaling a future automatically resolves it:

```go
type Response struct {
    Author *psql.Future[User] `json:"author"`
}

resp := Response{Author: psql.Lazy[User]("ID", authorID)}
data, err := json.Marshal(resp) // resolves the future, then marshals the User
```

### Not Found

If no record matches the future's value, `Resolve` returns `os.ErrNotExist`:

```go
user, err := future.Resolve(ctx)
if errors.Is(err, os.ErrNotExist) {
    // record not found
}
```

## Change Detection

`HasChanged` reports whether a loaded object has been modified since it was last fetched or saved:

```go
user, _ := psql.Get[User](ctx, map[string]any{"ID": uint64(1)})

fmt.Println(psql.HasChanged(user)) // false

user.Name = "New Name"
fmt.Println(psql.HasChanged(user)) // true
```

This compares current field values against the state captured during the last database scan. Objects that were never loaded from the database always report as changed.
