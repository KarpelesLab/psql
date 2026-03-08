// Package psql provides a multi-engine SQL ORM and query builder for Go.
//
// It supports MySQL, PostgreSQL (including CockroachDB), and SQLite with
// automatic table creation, object binding via struct tags, lifecycle hooks,
// association preloading, vector similarity search, and transactions.
//
// # Connecting
//
// Use [New] to connect with automatic engine detection from the DSN:
//
//	be, err := psql.New("postgresql://user:pass@localhost:5432/mydb")
//	be, err := psql.New("user:pass@tcp(localhost:3306)/mydb")
//	be, err := psql.New("sqlite:mydata.db")
//
// Attach the backend to a context for all subsequent operations:
//
//	ctx := be.Plug(context.Background())
//
// # Defining Tables
//
// Map Go structs to database tables using struct tags. Embed [Name] with an
// sql tag to set the table name explicitly:
//
//	type User struct {
//	    psql.Name `sql:"users"`
//	    ID        uint64 `sql:",key=PRIMARY"`
//	    Email     string `sql:",type=VARCHAR,size=255"`
//	    Name      string `sql:",type=VARCHAR,size=128"`
//	}
//
// The sql tag format is "[column_name],attr=value,attr=value,...". Supported
// attributes include type, size, scale, key, default, and values (for enums).
// Use sql:"-" to exclude a field. Pointer types are automatically nullable.
//
// Tables are created or updated automatically on first use to match the
// struct definition.
//
// # CRUD Operations
//
// The package provides generic functions for common operations:
//
//	// Insert one or more records
//	err := psql.Insert(ctx, &User{ID: 1, Name: "Alice"})
//
//	// Get a single record by key
//	user, err := psql.Get[User](ctx, map[string]any{"ID": uint64(1)})
//
//	// Fetch multiple records
//	users, err := psql.Fetch[User](ctx, map[string]any{"Name": "Alice"})
//
//	// Update a record (only changed fields)
//	user.Name = "Alice Smith"
//	err = psql.Update(ctx, user)
//
//	// Replace (upsert)
//	err = psql.Replace(ctx, user)
//
//	// Delete
//	_, err = psql.Delete[User](ctx, map[string]any{"ID": uint64(1)})
//
//	// Count
//	n, err := psql.Count[User](ctx, map[string]any{"Name": "Alice"})
//
// # Fetch Options
//
// Control query behavior with [FetchOptions]:
//
//	users, _ := psql.Fetch[User](ctx, nil, psql.Limit(10))
//	users, _ := psql.Fetch[User](ctx, nil, psql.LimitFrom(20, 10))
//	users, _ := psql.Fetch[User](ctx, nil, psql.Sort(psql.S("Name", "ASC")))
//	users, _ := psql.Fetch[User](ctx, nil, psql.FetchLock) // SELECT FOR UPDATE
//
// # Iterators
//
// Go 1.23 range iterators are supported via [Iter]:
//
//	iter, err := psql.Iter[User](ctx, map[string]any{"Active": true})
//	for user := range iter {
//	    fmt.Println(user.Name)
//	}
//
// # Hooks
//
// Implement hook interfaces on your struct for lifecycle callbacks:
//
//   - [BeforeSaveHook] — fires before Insert, Update, and Replace
//   - [AfterSaveHook] — fires after Insert, Update, and Replace
//   - [BeforeInsertHook] / [AfterInsertHook] — fires on Insert and InsertIgnore
//   - [BeforeUpdateHook] / [AfterUpdateHook] — fires on Update
//   - [AfterScanHook] — fires after scanning a row from any fetch operation
//
// Returning an error from a "Before" hook prevents the operation. Example:
//
//	func (u *User) BeforeInsert(ctx context.Context) error {
//	    if u.CreatedAt.IsZero() {
//	        u.CreatedAt = time.Now()
//	    }
//	    return nil
//	}
//
// # Associations
//
// Declare relationships using the psql struct tag (separate from sql):
//
//	type Book struct {
//	    psql.Name `sql:"books"`
//	    ID        int64   `sql:",key=PRIMARY"`
//	    AuthorID  int64   `sql:",type=BIGINT"`
//	    Author    *Author `psql:"belongs_to:AuthorID"`
//	}
//
//	type Author struct {
//	    psql.Name `sql:"authors"`
//	    ID        int64   `sql:",key=PRIMARY"`
//	    Books     []*Book `psql:"has_many:AuthorID"`
//	    Profile   *Profile `psql:"has_one:AuthorID"`
//	}
//
// Use [Preload] or [WithPreload] to batch-load associations efficiently:
//
//	books, _ := psql.Fetch[Book](ctx, nil, psql.WithPreload("Author"))
//
// # Query Builder
//
// Build SQL queries programmatically with [B]:
//
//	rows, err := psql.B().
//	    Select("id", "name").
//	    From("users").
//	    Where(psql.Equal(psql.F("status"), "active")).
//	    OrderBy(psql.S("name", "ASC")).
//	    Limit(50).
//	    RunQuery(ctx)
//
// Use [F] for field references, [V] for value literals, [S] for sort fields,
// and [Raw] for raw SQL fragments.
//
// # Transactions
//
// Use [Tx] for callback-based transactions or [BeginTx] for manual control:
//
//	err := psql.Tx(ctx, func(ctx context.Context) error {
//	    psql.Insert(ctx, &user)
//	    psql.Insert(ctx, &profile)
//	    return nil // commit; return error to rollback
//	})
//
// Nested transactions are supported via SQL savepoints. To run a query
// outside the current transaction (e.g., logging a failure before rolling
// back), use the original pre-transaction context or call [EscapeTx]:
//
//	outerCtx := ctx
//	psql.Tx(ctx, func(ctx context.Context) error {
//	    if err := psql.Insert(ctx, &order); err != nil {
//	        psql.Insert(outerCtx, &AuditLog{Event: "failed"}) // persists after rollback
//	        return err
//	    }
//	    return nil
//	})
//
// # Vector Support
//
// Use the [Vector] type for similarity search with PostgreSQL pgvector or
// CockroachDB native vectors:
//
//	type Item struct {
//	    psql.Name `sql:"items"`
//	    ID        uint64 `sql:",key=PRIMARY"`
//	    Embedding Vector `sql:",type=VECTOR,size=384"`
//	}
//
// Distance functions: [VecL2Distance], [VecCosineDistance], [VecInnerProduct].
package psql
