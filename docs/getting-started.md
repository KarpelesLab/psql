# Getting Started

## Installation

```bash
go get github.com/KarpelesLab/psql
```

Then install the driver submodule for your database:

```bash
go get github.com/KarpelesLab/psql/mysql   # MySQL / MariaDB
go get github.com/KarpelesLab/psql/pgsql   # PostgreSQL / CockroachDB
go get github.com/KarpelesLab/psql/sqlite  # SQLite
```

Requires Go 1.23 or later.

## Connecting to a Database

Import the driver with a blank identifier, then use `psql.New()` with a DSN string. The engine is auto-detected from the DSN format:

```go
import (
    "github.com/KarpelesLab/psql"
    _ "github.com/KarpelesLab/psql/pgsql"  // register PostgreSQL driver
)

be, err := psql.New("postgresql://user:pass@localhost:5432/mydb")
```

### DSN Formats

```go
import _ "github.com/KarpelesLab/psql/pgsql"
be, err := psql.New("postgresql://user:pass@localhost:5432/mydb")  // PostgreSQL
be, err := psql.New("postgres://user:pass@localhost:5432/mydb")    // also PostgreSQL

import _ "github.com/KarpelesLab/psql/mysql"
be, err := psql.New("user:pass@tcp(localhost:3306)/mydb")          // MySQL

import _ "github.com/KarpelesLab/psql/sqlite"
be, err := psql.New(":memory:")           // SQLite in-memory
be, err := psql.New("sqlite:mydata.db")   // SQLite file
be, err := psql.New("file:test.db")       // SQLite file URI
be, err := psql.New("data.sqlite3")       // detected by extension
```

### Engine-Specific Constructors

Each submodule also exports a `New` function for engine-specific configuration:

```go
import psqlmysql "github.com/KarpelesLab/psql/mysql"

cfg, _ := mysql.ParseDSN("user:pass@tcp(localhost:3306)/mydb")
be, err := psqlmysql.New(cfg)
```

```go
import psqlpg "github.com/KarpelesLab/psql/pgsql"

cfg, _ := pgxpool.ParseConfig("postgresql://...")
be, err := psqlpg.New(cfg)
```

## Attaching to Context

All psql operations use `context.Context` to find the database backend. Attach the backend to your context:

```go
ctx := be.Plug(context.Background())
// or equivalently:
ctx := psql.ContextBackend(context.Background(), be)
```

All subsequent operations using this context will use the attached backend.

## Defining a Table

Define a Go struct with `sql` tags to map it to a database table:

```go
type User struct {
    psql.Name `sql:"users"`                          // explicit table name
    ID        uint64 `sql:",key=PRIMARY"`             // primary key
    Email     string `sql:",type=VARCHAR,size=255"`   // VARCHAR(255)
    Name      string `sql:",type=VARCHAR,size=128"`   // VARCHAR(128)
    Age       int    `sql:",type=INT"`                // INT
}
```

Tables are automatically created or updated when first used. The library compares the struct definition with the existing table schema and applies any necessary changes (adding columns, creating indexes, etc.).

## Basic CRUD Operations

```go
// Insert
err := psql.Insert(ctx, &User{ID: 1, Email: "alice@example.com", Name: "Alice", Age: 30})

// Get (single record by key)
user, err := psql.Get[User](ctx, map[string]any{"ID": uint64(1)})

// Fetch (multiple records)
users, err := psql.Fetch[User](ctx, map[string]any{"Age": 30})

// Update
user.Name = "Alice Smith"
err = psql.Update(ctx, user)

// Replace (upsert)
err = psql.Replace(ctx, &User{ID: 1, Email: "alice@new.com", Name: "Alice", Age: 31})

// InsertIgnore (skip on conflict)
err = psql.InsertIgnore(ctx, &User{ID: 1, Name: "Alice"})

// Delete
_, err = psql.Delete[User](ctx, map[string]any{"ID": uint64(1)})

// Count
count, err := psql.Count[User](ctx, map[string]any{"Age": 30})

// HasChanged (detect modifications since last load)
changed := psql.HasChanged(user)
```

## Error Helpers

```go
// Check for duplicate key violations (works across all engines)
if psql.IsDuplicate(err) {
    // unique constraint violated
}

// Check for missing table/column errors
if psql.IsNotExist(err) {
    // table or column doesn't exist
}
```

## Supported Engines

| Engine | Driver Submodule | DSN Prefix |
|--------|-----------------|------------|
| MySQL / MariaDB | `psql/mysql` | `user:pass@tcp(...)` |
| PostgreSQL | `psql/pgsql` | `postgresql://` or `postgres://` |
| CockroachDB | `psql/pgsql` | `postgresql://` (uses PostgreSQL driver) |
| SQLite | `psql/sqlite` | `:memory:`, `sqlite:`, file extension |

## Next Steps

- [Object Binding](object-binding.md) - Detailed struct tag reference
- [Hooks](hooks.md) - Lifecycle callbacks
- [Associations](associations.md) - Relationships between models
- [Query Builder](query-builder.md) - Building complex SQL queries
- [Transactions](transactions.md) - Transaction support
- [Vectors](vectors.md) - Vector similarity search
- [Scopes & Lazy](scopes-lazy.md) - Reusable query modifiers and lazy loading
- [Soft Delete](soft-delete.md) - Automatic soft delete
- [Naming Strategies](naming-strategies.md) - Customizing table/column names
