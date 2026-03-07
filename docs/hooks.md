# Hooks

Hooks are lifecycle callbacks that fire before or after database operations. Implement hook interfaces on your struct to add custom logic.

## Available Hooks

| Interface | Method | Fires On |
|-----------|--------|----------|
| `BeforeSaveHook` | `BeforeSave(ctx context.Context) error` | Insert, Update, Replace |
| `AfterSaveHook` | `AfterSave(ctx context.Context) error` | Insert, Update, Replace |
| `BeforeInsertHook` | `BeforeInsert(ctx context.Context) error` | Insert, InsertIgnore |
| `AfterInsertHook` | `AfterInsert(ctx context.Context) error` | Insert, InsertIgnore |
| `BeforeUpdateHook` | `BeforeUpdate(ctx context.Context) error` | Update |
| `AfterUpdateHook` | `AfterUpdate(ctx context.Context) error` | Update |
| `AfterScanHook` | `AfterScan(ctx context.Context) error` | Get, Fetch, FetchOne, Iter |

## Execution Order

### Insert / InsertIgnore

```
BeforeSave -> BeforeInsert -> [SQL INSERT] -> AfterInsert -> AfterSave
```

### Update

```
BeforeSave -> BeforeUpdate -> [SQL UPDATE] -> AfterUpdate -> AfterSave
```

### Replace

```
BeforeSave -> [SQL REPLACE/UPSERT] -> AfterSave
```

### Fetch / Get / FetchOne

```
[SQL SELECT] -> [scan row] -> AfterScan
```

## Example: Setting Defaults

```go
type Article struct {
    psql.Name `sql:"articles"`
    ID        uint64    `sql:",key=PRIMARY"`
    Title     string    `sql:",type=VARCHAR,size=256"`
    Slug      string    `sql:",type=VARCHAR,size=256"`
    CreatedAt time.Time `sql:",type=DATETIME"`
}

func (a *Article) BeforeInsert(ctx context.Context) error {
    if a.Slug == "" {
        a.Slug = slugify(a.Title)
    }
    if a.CreatedAt.IsZero() {
        a.CreatedAt = time.Now()
    }
    return nil
}
```

The hook modifies the struct before the SQL is executed, so the changes are persisted to the database.

## Example: Validation

```go
var ErrInvalidEmail = errors.New("invalid email address")

type User struct {
    psql.Name `sql:"users"`
    ID        uint64 `sql:",key=PRIMARY"`
    Email     string `sql:",type=VARCHAR,size=255"`
}

func (u *User) BeforeSave(ctx context.Context) error {
    if !strings.Contains(u.Email, "@") {
        return ErrInvalidEmail
    }
    return nil
}
```

Returning an error from a "Before" hook prevents the database operation. The error propagates to the caller.

## Example: Audit Logging

```go
func (u *User) AfterUpdate(ctx context.Context) error {
    slog.Info("user updated", "user_id", u.ID)
    return nil
}
```

## Example: Post-Load Processing

```go
type Config struct {
    psql.Name `sql:"configs"`
    ID        uint64 `sql:",key=PRIMARY"`
    RawJSON   string `sql:",type=TEXT"`
    Parsed    map[string]any `sql:"-"` // not stored in DB
}

func (c *Config) AfterScan(ctx context.Context) error {
    return json.Unmarshal([]byte(c.RawJSON), &c.Parsed)
}
```

`AfterScan` runs after every row is loaded from the database. Returning an error prevents the object from being returned (Get and Fetch will return the error).

## Error Handling

- **Before hooks**: Returning an error prevents the database operation entirely. The row is not written.
- **After hooks**: Returning an error is propagated to the caller. For Insert/Update, the database operation has already completed.
- **AfterScan**: Returning an error prevents the scanned object from being returned to the caller.

## Batch Operations

When inserting or updating multiple objects, hooks are called individually for each object:

```go
err := psql.Insert(ctx, &obj1, &obj2, &obj3)
// Calls BeforeSave+BeforeInsert on obj1, then INSERT obj1, then AfterInsert+AfterSave on obj1
// Then the same for obj2, then obj3
```

If any hook returns an error, the operation stops at that point. Previously inserted objects in the batch are not rolled back (use transactions for atomicity).
