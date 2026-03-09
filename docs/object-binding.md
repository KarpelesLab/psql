# Object Binding

psql maps Go structs to database tables using struct tags. Tables are automatically created and kept in sync with the struct definition.

## Table Name

Set the table name by embedding `psql.Name` with an `sql` tag:

```go
type User struct {
    psql.Name `sql:"users"`  // table name is "users"
    // fields...
}
```

If no `psql.Name` is embedded, the table name is derived from the struct name using the configured [naming strategy](naming-strategies.md).

## Column Tags

Fields are mapped to columns using the `sql` struct tag:

```go
type Product struct {
    psql.Name   `sql:"products"`
    ID          uint64  `sql:",key=PRIMARY"`
    ProductName string  `sql:"Name,type=VARCHAR,size=128"`  // explicit column name "Name"
    Price       float64 `sql:",type=DECIMAL,size=10,scale=2"`
    Description *string `sql:",type=TEXT"`                   // nullable (pointer type)
    Hidden      bool    `sql:"-"`                            // excluded from database
}
```

### Tag Format

The `sql` tag format is: `"[column_name],key1=value1,key2=value2,..."`

- The first element (before the first comma) is an optional explicit column name
- If omitted or empty (e.g., `sql:",type=INT"`), the Go field name is used
- Use `sql:"-"` to exclude a field entirely

### Supported Tag Attributes

| Attribute | Description | Example |
|-----------|-------------|---------|
| `type` | SQL column type | `type=VARCHAR`, `type=INT`, `type=BIGINT` |
| `size` | Column size/length | `size=255` |
| `scale` | Decimal scale | `scale=2` |
| `key` | Key/index name | `key=PRIMARY`, `key=UNIQUE` |
| `default` | Default value | `default=0` |
| `values` | Enum values (comma-separated) | `values=active,inactive,pending` |
| `import` | Auto-detected from Go type | (set automatically if no attributes) |

### Column Types

When no explicit `type` is set, psql infers the SQL type from the Go type:

| Go Type | SQL Type |
|---------|----------|
| `string` | `VARCHAR(255)` |
| `int`, `int64` | `BIGINT` |
| `int32` | `INT` |
| `uint64` | `BIGINT UNSIGNED` |
| `float32` | `FLOAT` |
| `float64` | `DOUBLE` |
| `bool` | `TINYINT(1)` / `BOOLEAN` |
| `time.Time` | `DATETIME` / `TIMESTAMP` |
| `[]byte` | `BLOB` |
| pointer types | Same as base type, but `NULL`able |

## Keys and Indexes

### Primary Key

```go
type User struct {
    ID uint64 `sql:",key=PRIMARY"`
}
```

### Composite Primary Key

```go
type UserRole struct {
    UserID uint64 `sql:",key=PRIMARY"`
    RoleID uint64 `sql:",key=PRIMARY"`
}
```

### Unique Index

```go
type User struct {
    ID    uint64 `sql:",key=PRIMARY"`
    Email string `sql:",type=VARCHAR,size=255,key=UNIQUE"`
}
```

### Named Index

```go
type Event struct {
    ID        uint64 `sql:",key=PRIMARY"`
    UserID    uint64 `sql:",key=idx_user_date"`
    EventDate string `sql:",type=DATE,key=idx_user_date"`
}
```

## Enum Support

Define enum columns using a string type:

```go
type StatusEnum string

const (
    StatusPending  StatusEnum = "pending"
    StatusActive   StatusEnum = "active"
    StatusInactive StatusEnum = "inactive"
)

type Account struct {
    psql.Name `sql:"accounts"`
    ID        uint64     `sql:",key=PRIMARY"`
    Status    StatusEnum `sql:",type=enum,values=pending,active,inactive"`
}
```

- **MySQL**: Creates a native `ENUM` column
- **PostgreSQL**: Creates a custom type with a `CHECK` constraint
- **SQLite**: Uses a `CHECK` constraint for validation

## Custom Types

### Hex

`psql.Hex` stores `[]byte` data as hexadecimal strings:

```go
type Token struct {
    ID   uint64   `sql:",key=PRIMARY"`
    Hash psql.Hex `sql:",type=BINARY,size=32"`
}
```

### Set

`psql.Set` stores a `[]string` as a comma-separated `SET` column (MySQL) or equivalent:

```go
type User struct {
    ID    uint64   `sql:",key=PRIMARY"`
    Perms psql.Set `sql:",type=set,values=read,write,admin"`
}
```

### Vector

`psql.Vector` stores float32 vectors for similarity search. See [Vectors](vectors.md) for details.

## Registering Tables

Tables are automatically registered on first use. You can also explicitly register them:

```go
_ = psql.Table[User]()      // registers and returns the table metadata
_ = psql.Table[Product]()
```

This is useful for ensuring tables exist before they're needed, or for registering association target types.

## FetchOne

`FetchOne` scans into an existing variable instead of allocating a new one:

```go
var user User
err := psql.FetchOne(ctx, &user, map[string]any{"Email": "alice@example.com"})
```

## Iterators (Go 1.23+)

Use `Iter` to process results one at a time without loading all rows into memory:

```go
iter, err := psql.Iter[User](ctx, map[string]any{"Age": 30})
if err != nil {
    return err
}
for user := range iter {
    fmt.Println(user.Name)
}
```

## Batch Operations

Insert, Update, and Replace accept multiple objects:

```go
err := psql.Insert(ctx,
    &User{ID: 1, Name: "Alice"},
    &User{ID: 2, Name: "Bob"},
    &User{ID: 3, Name: "Charlie"},
)
```

## Fetch Options

Control query behavior with `FetchOptions`:

```go
// Limit results
users, err := psql.Fetch[User](ctx, nil, psql.Limit(10))

// Limit with offset
users, err := psql.Fetch[User](ctx, nil, psql.LimitFrom(20, 10))

// Sort results
users, err := psql.Fetch[User](ctx, nil, psql.Sort(psql.S("Name", "ASC")))

// Lock rows (SELECT FOR UPDATE)
users, err := psql.Fetch[User](ctx, nil, psql.FetchLock)

// FOR UPDATE SKIP LOCKED (skip rows locked by other transactions)
users, err := psql.Fetch[User](ctx, nil, psql.FetchLockSkipLocked)

// FOR UPDATE NOWAIT (fail immediately if rows are locked)
users, err := psql.Fetch[User](ctx, nil, psql.FetchLockNoWait)

// Include soft-deleted records
users, err := psql.Fetch[User](ctx, nil, psql.IncludeDeleted())

// Apply reusable scopes
users, err := psql.Fetch[User](ctx, nil, psql.WithScope(Active))

// Combine options
users, err := psql.Fetch[User](ctx,
    map[string]any{"Age": 30},
    psql.Sort(psql.S("Name", "ASC")),
    psql.Limit(10),
)
```

## Mapped and Grouped Fetching

```go
// FetchMapped returns a map keyed by a column value
userMap, err := psql.FetchMapped[User](ctx, nil, "Email")
// userMap["alice@example.com"] = &User{...}

// FetchGrouped returns a map of slices grouped by a column value
grouped, err := psql.FetchGrouped[User](ctx, nil, "Age")
// grouped["30"] = []*User{...}
```
