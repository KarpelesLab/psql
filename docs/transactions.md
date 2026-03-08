# Transactions

psql supports transactions through context propagation, including nested transactions via savepoints.

## Basic Transactions

### Callback Style

The simplest way to use transactions:

```go
err := psql.Tx(ctx, func(ctx context.Context) error {
    err := psql.Insert(ctx, &User{ID: 1, Name: "Alice"})
    if err != nil {
        return err // triggers rollback
    }
    err = psql.Insert(ctx, &Profile{ID: 1, UserID: 1, Bio: "Hello"})
    if err != nil {
        return err // triggers rollback
    }
    return nil // triggers commit
})
```

`Tx` automatically commits if the callback returns `nil`, or rolls back if it returns an error.

### Manual Style

For more control:

```go
tx, err := psql.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback() // safe to call even after commit

ctx = psql.ContextTx(ctx, tx)

err = psql.Insert(ctx, &User{ID: 1, Name: "Alice"})
if err != nil {
    return err // deferred rollback will execute
}

return tx.Commit()
```

## Nested Transactions

psql supports nested transactions using SQL savepoints. Starting a transaction inside an existing one creates a savepoint:

```go
err := psql.Tx(ctx, func(ctx context.Context) error {
    psql.Insert(ctx, &User{ID: 1, Name: "Alice"})

    // Nested transaction (savepoint)
    err := psql.Tx(ctx, func(ctx context.Context) error {
        psql.Insert(ctx, &User{ID: 2, Name: "Bob"})
        return errors.New("oops") // rolls back to savepoint, Bob is NOT inserted
    })
    // err is non-nil but we can continue

    psql.Insert(ctx, &User{ID: 3, Name: "Charlie"})
    return nil // commits: Alice and Charlie are saved
})
```

Nested transactions are implemented as `SAVEPOINT`/`ROLLBACK TO` statements, which work across all supported engines.

## Transaction Options

Pass `*sql.TxOptions` to control isolation level and read-only mode:

```go
tx, err := psql.BeginTx(ctx, &sql.TxOptions{
    Isolation: sql.LevelSerializable,
    ReadOnly:  true,
})
```

## Safe Deletion

`DeleteOne` wraps the deletion in a transaction and verifies exactly one row was affected:

```go
err := psql.DeleteOne[User](ctx, map[string]any{"ID": uint64(1)})
// Returns an error if 0 or 2+ rows would be deleted
```

## Running Queries Outside a Transaction

Since psql routes queries based on context, you can run queries outside
the current transaction by using a context that doesn't carry the
transaction. This is useful for operations that must persist regardless
of whether the transaction commits or rolls back, such as logging
failures before a rollback.

### Keeping the Original Context

The simplest approach is to keep a reference to the pre-transaction context:

```go
// outerCtx has the backend but no transaction
outerCtx := ctx

err := psql.Tx(ctx, func(ctx context.Context) error {
    // ctx is inside the transaction
    err := psql.Insert(ctx, &Order{ID: 1, Status: "pending"})
    if err != nil {
        // Log the failure outside the transaction — this INSERT commits
        // immediately and survives the rollback that follows.
        psql.Insert(outerCtx, &AuditLog{Event: "order_insert_failed", Detail: err.Error()})
        return err // triggers rollback
    }
    return nil
})
```

Because `outerCtx` was captured before `Tx` wrapped the context with a
transaction, any query using it goes directly to the database connection
pool, completely independent of the transaction's fate.

### Using EscapeTx

When you only have access to the transactional context (e.g., inside a
hook or a helper function), use [EscapeTx] to obtain the underlying
non-transactional context:

```go
func logEvent(ctx context.Context, event string) {
    outerCtx, ok := psql.EscapeTx(ctx)
    if ok {
        // outerCtx is outside the transaction
        psql.Insert(outerCtx, &AuditLog{Event: event})
    } else {
        // no transaction was active, use ctx directly
        psql.Insert(ctx, &AuditLog{Event: event})
    }
}
```

`EscapeTx` walks up the context chain and returns the context just
below the transaction layer, preserving the backend and any other values
attached above the transaction.
