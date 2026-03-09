# Soft Delete

psql automatically enables soft delete when a struct has a nullable `*time.Time` field named `DeletedAt` (or mapped to a column that follows the naming convention). Instead of removing rows, `Delete` sets the timestamp; queries automatically exclude soft-deleted records.

## Defining a Soft-Delete Table

```go
type Post struct {
    psql.Name `sql:"posts"`
    ID        uint64     `sql:",key=PRIMARY"`
    Title     string     `sql:",type=VARCHAR,size=256"`
    DeletedAt *time.Time `sql:",type=DATETIME"`  // enables soft delete
}
```

The soft-delete field is auto-detected: any `*time.Time` field works. The library checks for fields that can hold a deletion timestamp.

## How It Works

### Delete (Soft)

`Delete` sets `DeletedAt` to the current time instead of removing the row:

```go
_, err := psql.Delete[Post](ctx, map[string]any{"ID": uint64(1)})
// UPDATE "posts" SET "DeletedAt"=? WHERE "ID"=? AND "DeletedAt" IS NULL
```

The `AND "DeletedAt" IS NULL` condition prevents double-deleting already-deleted records.

### Automatic Filtering

All queries automatically exclude soft-deleted records:

```go
posts, _ := psql.Fetch[Post](ctx, nil)
// SELECT ... FROM "posts" WHERE "DeletedAt" IS NULL

count, _ := psql.Count[Post](ctx, nil)
// SELECT COUNT(1) FROM "posts" WHERE "DeletedAt" IS NULL
```

### Include Soft-Deleted Records

Use `IncludeDeleted()` to bypass the automatic filter:

```go
allPosts, _ := psql.Fetch[Post](ctx, nil, psql.IncludeDeleted())
// SELECT ... FROM "posts" (no DeletedAt filter)
```

### Restore

Un-delete records by clearing the `DeletedAt` timestamp:

```go
_, err := psql.Restore[Post](ctx, map[string]any{"ID": uint64(1)})
// UPDATE "posts" SET "DeletedAt"=NULL WHERE "ID"=?
```

Returns `ErrNotReady` if the table has no soft-delete field.

### Force Delete (Hard)

Permanently remove a record, bypassing soft delete:

```go
_, err := psql.ForceDelete[Post](ctx, map[string]any{"ID": uint64(1)})
// DELETE FROM "posts" WHERE "ID"=?
```

## Notes

- Soft delete is enabled automatically when a `*time.Time` field is detected; no configuration is needed.
- Tables without a `*time.Time` field use normal hard deletes.
- `DeleteOne` (transactional single-row delete) also respects soft delete.
- Soft-delete filtering applies to `Fetch`, `Get`, `FetchOne`, `Count`, `Iter`, and `Delete`.
- `ForceDelete` and queries with `IncludeDeleted()` bypass the filter.
