# Getting Started

## Installation

```bash
go get github.com/KarpelesLab/psql
```

Requires Go 1.23 or later.

## Connecting to a Database

Use `psql.New()` with a DSN string. The engine is auto-detected from the DSN format:

```go
// PostgreSQL / CockroachDB
be, err := psql.New("postgresql://user:pass@localhost:5432/mydb")

// MySQL
be, err := psql.New("user:pass@tcp(localhost:3306)/mydb")

// SQLite
be, err := psql.New("sqlite:mydata.db")
be, err := psql.New(":memory:")        // in-memory database
be, err := psql.New("file:test.db")    // file URI
be, err := psql.New("data.sqlite3")    // detected by extension
```

You can also use engine-specific constructors:

```go
// MySQL with custom config
cfg, _ := mysql.ParseDSN("user:pass@tcp(localhost:3306)/mydb")
be, err := psql.NewMySQL(cfg)

// PostgreSQL with pgxpool config
cfg, _ := pgxpool.ParseConfig("postgresql://...")
be, err := psql.NewPG(cfg)

// SQLite
be, err := psql.NewSQLite("mydata.db")
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

// Delete
_, err = psql.Delete[User](ctx, map[string]any{"ID": uint64(1)})

// Count
count, err := psql.Count[User](ctx, map[string]any{"Age": 30})
```

## Supported Engines

| Engine | Status | Notes |
|--------|--------|-------|
| MySQL | Full support | Default engine for plain DSN strings |
| PostgreSQL | Full support | Auto-detected by `postgresql://` prefix |
| CockroachDB | Full support | Uses PostgreSQL driver, compatible with PostgreSQL engine |
| SQLite | Full support | Auto-detected by file extension or `sqlite:` prefix |

## Next Steps

- [Object Binding](object-binding.md) - Detailed struct tag reference
- [Hooks](hooks.md) - Lifecycle callbacks
- [Associations](associations.md) - Relationships between models
- [Query Builder](query-builder.md) - Building complex SQL queries
- [Transactions](transactions.md) - Transaction support
- [Vectors](vectors.md) - Vector similarity search
- [Naming Strategies](naming-strategies.md) - Customizing table/column names
