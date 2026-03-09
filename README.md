[![Go Reference](https://pkg.go.dev/badge/github.com/portablesql/psql.svg)](https://pkg.go.dev/github.com/portablesql/psql)
[![Build Status](https://github.com/portablesql/psql/actions/workflows/test.yml/badge.svg)](https://github.com/portablesql/psql/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/portablesql/psql/badge.svg?branch=master)](https://coveralls.io/github/portablesql/psql?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/portablesql/psql)](https://goreportcard.com/report/github.com/portablesql/psql)

# psql

Platform SQL library for Go with object binding, query builder, hooks, associations, and vector support. Works with MySQL, PostgreSQL, CockroachDB, and SQLite.

Similar to GORM but focused on modern Go features (generics, 1.23 iterators) and a lighter footprint.

## Quick Start

```go
import (
    "github.com/portablesql/psql"
    _ "github.com/portablesql/psql-sqlite" // import the driver you need
)

// Connect (engine auto-detected from DSN)
be, err := psql.New(":memory:")
ctx := be.Plug(context.Background())

// Define a table
type User struct {
    psql.Name `sql:"users"`
    ID        uint64 `sql:",key=PRIMARY"`
    Name      string `sql:",type=VARCHAR,size=128"`
    Email     string `sql:",type=VARCHAR,size=255"`
}

// CRUD operations
err = psql.Insert(ctx, &User{ID: 1, Name: "Alice", Email: "alice@example.com"})

user, err := psql.Get[User](ctx, map[string]any{"ID": uint64(1)})

user.Name = "Alice Smith"
err = psql.Update(ctx, user)

users, err := psql.Fetch[User](ctx, map[string]any{"Name": "Alice Smith"})
```

## Features

### Multi-Engine Support

Import only the driver submodule you need:

```go
import _ "github.com/portablesql/psql-mysql"   // MySQL / MariaDB
import _ "github.com/portablesql/psql-pgsql"   // PostgreSQL / CockroachDB
import _ "github.com/portablesql/psql-sqlite"  // SQLite
```

DSN format is auto-detected:

```go
be, _ := psql.New("postgresql://...")        // PostgreSQL / CockroachDB
be, _ := psql.New("user:pass@tcp(...)/db")   // MySQL
be, _ := psql.New(":memory:")                // SQLite
```

### Hooks

Lifecycle callbacks via Go interfaces:

```go
func (u *User) BeforeInsert(ctx context.Context) error {
    u.CreatedAt = time.Now()
    return nil
}

func (u *User) BeforeSave(ctx context.Context) error {
    if !strings.Contains(u.Email, "@") {
        return errors.New("invalid email")
    }
    return nil
}
```

Available hooks: `BeforeSave`, `AfterSave`, `BeforeInsert`, `AfterInsert`, `BeforeUpdate`, `AfterUpdate`, `AfterScan`.

### Associations

Declare relationships and batch-preload to avoid N+1 queries:

```go
type Book struct {
    psql.Name `sql:"books"`
    ID        int64   `sql:",key=PRIMARY"`
    AuthorID  int64   `sql:",type=BIGINT"`
    Title     string  `sql:",type=VARCHAR,size=256"`
    Author    *Author `psql:"belongs_to:AuthorID"`
}

// Fetch books with authors preloaded (2 queries, not N+1)
books, err := psql.Fetch[Book](ctx, nil, psql.WithPreload("Author"))
```

Supports `belongs_to`, `has_one`, `has_many`, and `many_to_many`.

### Scopes

Reusable query modifiers:

```go
var Active psql.Scope = func(q *psql.QueryBuilder) *psql.QueryBuilder {
    return q.Where(map[string]any{"Status": "active"})
}

func RecentN(n int) psql.Scope {
    return func(q *psql.QueryBuilder) *psql.QueryBuilder {
        return q.OrderBy(psql.S("CreatedAt", "DESC")).Limit(n)
    }
}

users, err := psql.Fetch[User](ctx, nil, psql.WithScope(Active, RecentN(10)))
```

### Lazy Loading

Batch-optimized deferred queries:

```go
future1 := psql.Lazy[User]("ID", "1")
future2 := psql.Lazy[User]("ID", "2")

// Resolving any future batches all pending futures into a single IN query
user1, err := future1.Resolve(ctx)
user2, err := future2.Resolve(ctx) // already resolved by the batch
```

### Soft Delete

Automatic soft delete with timestamp columns:

```go
type Post struct {
    psql.Name `sql:"posts"`
    ID        uint64     `sql:",key=PRIMARY"`
    Title     string     `sql:",type=VARCHAR,size=256"`
    DeletedAt *time.Time `sql:",type=DATETIME"`  // soft delete field (auto-detected)
}

psql.Delete[Post](ctx, map[string]any{"ID": uint64(1)})  // sets DeletedAt
psql.Restore[Post](ctx, map[string]any{"ID": uint64(1)}) // clears DeletedAt
psql.ForceDelete[Post](ctx, map[string]any{"ID": uint64(1)}) // hard DELETE

// Include soft-deleted records
posts, _ := psql.Fetch[Post](ctx, nil, psql.IncludeDeleted())
```

### Iterators (Go 1.23+)

```go
iter, err := psql.Iter[User](ctx, map[string]any{"Status": "active"})
for user := range iter {
    fmt.Println(user.Name)
}
```

### Transactions

```go
err := psql.Tx(ctx, func(ctx context.Context) error {
    psql.Insert(ctx, &user)
    psql.Insert(ctx, &profile)
    return nil // commit; return error to rollback
})
```

Supports nested transactions via savepoints.

### Vector Similarity Search

```go
type Item struct {
    psql.Name `sql:"items"`
    ID        uint64      `sql:",key=PRIMARY"`
    Embedding psql.Vector `sql:",type=VECTOR,size=384"`
}

// Nearest neighbor search
query := psql.B().Select("*").From("items").
    OrderBy(psql.VecCosineDistance(psql.F("Embedding"), queryVec)).
    Limit(10)
```

### Query Builder

```go
query := psql.B().
    Select("id", "name").
    From("users").
    Where(
        psql.Equal(psql.F("status"), "active"),
        psql.Gte(psql.F("age"), 18),
    ).
    OrderBy(psql.S("name", "ASC")).
    Limit(50)

rows, err := query.RunQuery(ctx)
```

### Error Helpers

```go
err := psql.Insert(ctx, &duplicateRecord)
if psql.IsDuplicate(err) {
    // handle unique constraint violation (works across all engines)
}
if psql.IsNotExist(err) {
    // handle missing table/column
}
```

### Enum Support

```go
type StatusEnum string
const (
    StatusActive   StatusEnum = "active"
    StatusInactive StatusEnum = "inactive"
)

type Account struct {
    ID     uint64     `sql:",key=PRIMARY"`
    Status StatusEnum `sql:",type=enum,values=active,inactive"`
}
```

## Documentation

| Topic | Description |
|-------|-------------|
| [Getting Started](docs/getting-started.md) | Installation, connecting, driver imports, basic CRUD |
| [Object Binding](docs/object-binding.md) | Struct tags, column types, keys, enums, custom types |
| [Hooks](docs/hooks.md) | Lifecycle callbacks, execution order, validation |
| [Associations](docs/associations.md) | belongs_to, has_one, has_many, many_to_many, preloading |
| [Query Builder](docs/query-builder.md) | SELECT, WHERE, JOIN, GROUP BY, subqueries, ON CONFLICT |
| [Transactions](docs/transactions.md) | Transactions, nested savepoints, safe deletion |
| [Vectors](docs/vectors.md) | Vector columns, similarity search, distance functions |
| [Naming Strategies](docs/naming-strategies.md) | DefaultNamer, CamelSnakeNamer, LegacyNamer |
| [Scopes & Lazy](docs/scopes-lazy.md) | Reusable scopes, lazy loading, change detection |
| [Soft Delete](docs/soft-delete.md) | Automatic soft delete, restore, force delete |
