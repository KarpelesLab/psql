# Naming Strategies

psql uses a `Namer` interface to control how Go names are mapped to SQL names for tables, columns, indexes, and other database objects.

## Setting a Naming Strategy

```go
be, _ := psql.New("postgresql://...")

// Use exact Go names (no transformation)
be.SetNamer(&psql.DefaultNamer{})

// Use Camel_Snake_Case for everything
be.SetNamer(&psql.CamelSnakeNamer{})

// Legacy behavior (default): Camel_Snake_Case for tables, no transform for columns
be.SetNamer(&psql.LegacyNamer{})
```

## Available Namers

### DefaultNamer

Keeps names exactly as they are:

| Go Name | Table Name | Column Name |
|---------|------------|-------------|
| `UserProfile` | `UserProfile` | `UserProfile` |
| `OrderItem` | `OrderItem` | `OrderItem` |

### CamelSnakeNamer

Converts all names to `Camel_Snake_Case`:

| Go Name | Table Name | Column Name |
|---------|------------|-------------|
| `UserProfile` | `User_Profile` | `User_Profile` |
| `OrderItem` | `Order_Item` | `Order_Item` |

### LegacyNamer (Default)

Table names use `Camel_Snake_Case`, column names are kept as-is:

| Go Name | Table Name | Column Name |
|---------|------------|-------------|
| `UserProfile` | `User_Profile` | `UserProfile` |
| `OrderItem` | `Order_Item` | `OrderItem` |

This is the default for backward compatibility.

## Explicit Names Override the Namer

When you set a name explicitly via `psql.Name`, it's used as-is regardless of the naming strategy:

```go
type User struct {
    psql.Name `sql:"my_users"` // always "my_users", namer is not applied
    ID        uint64 `sql:",key=PRIMARY"`
}
```

Similarly, explicit column names in sql tags are used directly:

```go
type User struct {
    FirstName string `sql:"first_name,type=VARCHAR,size=128"` // always "first_name"
}
```

## Namer Interface

All naming strategies implement the `Namer` interface:

```go
type Namer interface {
    TableName(table string) string
    SchemaName(table string) string
    ColumnName(table, column string) string
    JoinTableName(joinTable string) string
    CheckerName(table, column string) string
    IndexName(table, column string) string
    UniqueName(table, column string) string
    EnumTypeName(table, column string) string
}
```

You can implement your own `Namer` for custom naming conventions.
