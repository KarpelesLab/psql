# Associations

psql supports relationships between models using struct tags and batch preloading to avoid N+1 query problems.

## Association Types

### belongs_to

A record references a parent record via a foreign key column.

```go
type Author struct {
    psql.Name `sql:"authors"`
    ID        int64  `sql:",key=PRIMARY"`
    Name      string `sql:",type=VARCHAR,size=128"`
}

type Book struct {
    psql.Name `sql:"books"`
    ID        int64        `sql:",key=PRIMARY"`
    AuthorID  int64        `sql:",type=BIGINT"`
    Title     string       `sql:",type=VARCHAR,size=256"`
    Author    *Author      `psql:"belongs_to:AuthorID"` // FK is on this struct
}
```

`belongs_to:AuthorID` means: "This book belongs to an Author. The foreign key is `AuthorID` on this struct. Match it against the Author's primary key."

### has_one

A parent record has exactly one related child record.

```go
type Profile struct {
    psql.Name `sql:"profiles"`
    ID        int64  `sql:",key=PRIMARY"`
    AuthorID  int64  `sql:",type=BIGINT"`
    Bio       string `sql:",type=VARCHAR,size=512"`
}

type Author struct {
    psql.Name `sql:"authors"`
    ID        int64    `sql:",key=PRIMARY"`
    Name      string   `sql:",type=VARCHAR,size=128"`
    Profile   *Profile `psql:"has_one:AuthorID"` // FK is on the Profile struct
}
```

`has_one:AuthorID` means: "This author has one Profile. The foreign key `AuthorID` is on the Profile table. Match it against this Author's primary key."

### has_many

A parent record has multiple related child records.

```go
type Author struct {
    psql.Name `sql:"authors"`
    ID        int64    `sql:",key=PRIMARY"`
    Name      string   `sql:",type=VARCHAR,size=128"`
    Books     []*Book  `psql:"has_many:AuthorID"` // FK is on the Book struct
}
```

`has_many:AuthorID` means: "This author has many Books. The foreign key `AuthorID` is on the Book table. Match it against this Author's primary key."

**Important**: `has_many` fields must be slice types (e.g., `[]*Book`).

## Tag Format

Association tags use the `psql` struct tag (not `sql`):

```
psql:"<kind>:<ForeignKey>"
```

- `kind`: `belongs_to`, `has_one`, or `has_many`
- `ForeignKey`: The column name (or Go field name) of the foreign key

Association fields are excluded from the database schema -- they exist only in Go for loading related data.

## Preloading

### Explicit Preloading

After fetching records, use `Preload` to load their associations:

```go
books, err := psql.Fetch[Book](ctx, nil)
if err != nil {
    return err
}

// Load the Author for each book in a single query
err = psql.Preload(ctx, books, "Author")
```

You can preload multiple associations:

```go
err = psql.Preload(ctx, authors, "Books")
err = psql.Preload(ctx, authors, "Profile")
```

### Automatic Preloading with WithPreload

Use `WithPreload` as a fetch option to automatically preload associations:

```go
// Fetch + preload in one call
books, err := psql.Fetch[Book](ctx, nil, psql.WithPreload("Author"))

// Also works with Get
book, err := psql.Get[Book](ctx, map[string]any{"ID": int64(1)}, psql.WithPreload("Author"))

// And FetchOne
var book Book
err := psql.FetchOne(ctx, &book, map[string]any{"ID": int64(1)}, psql.WithPreload("Author"))
```

## How Preloading Works

Preloading is implemented as efficient batch loading using `IN` queries:

1. Collect all foreign key values from the loaded records
2. Execute a single `SELECT ... WHERE column IN (?, ?, ...)` query
3. Match results back to the parent records

This avoids the N+1 query problem. For example, loading 100 books and their authors only takes 2 queries (one for books, one for all referenced authors), not 101.

## Important Notes

### Table Registration

Both sides of an association must be registered before preloading:

```go
_ = psql.Table[Author]()   // register Author
_ = psql.Table[Book]()     // register Book
_ = psql.Table[Profile]()  // register Profile
```

Registration happens automatically when you first use a type with any psql operation (Insert, Fetch, etc.), but you may need to register types explicitly if they're only used as association targets.

### Nil Values

- **belongs_to**: If the FK value doesn't match any parent record, the field remains `nil`.
- **has_one**: If no related child record exists, the field remains `nil`.
- **has_many**: If no related child records exist, the slice remains `nil` (not an empty slice).

### Primary Key Requirement

- `belongs_to` requires the target type to have a single-column primary key.
- `has_one` and `has_many` require the parent type to have a single-column primary key.

### Empty Targets

Preloading an empty slice is a no-op and returns no error:

```go
err := psql.Preload[Book](ctx, nil, "Author") // returns nil
```
