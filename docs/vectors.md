# Vector Support

psql includes built-in support for vector columns and similarity search, compatible with PostgreSQL pgvector and CockroachDB native vector types.

## Defining Vector Columns

Use `psql.Vector` with the `VECTOR` type and specify the dimensions via `size`:

```go
type Item struct {
    psql.Name `sql:"items"`
    ID        uint64      `sql:",key=PRIMARY"`
    Title     string      `sql:",type=VARCHAR,size=256"`
    Embedding psql.Vector `sql:",type=VECTOR,size=384"` // 384-dimensional vector
}
```

`psql.Vector` is a `[]float32` that implements `sql.Scanner` and `driver.Valuer` for automatic serialization.

## Storing Vectors

```go
embedding := psql.Vector{0.1, 0.2, 0.3, ...} // your embedding from an ML model

err := psql.Insert(ctx, &Item{
    ID:        1,
    Title:     "Example",
    Embedding: embedding,
})
```

## Vector Comparison Operators

All five vector comparison operators are supported:

| Function | Description | SQL Operator |
|----------|-------------|-------------|
| `VecEqual` | Equality | `=` |
| `VecNotEqual` | Inequality | `<>` |
| `VecL2Distance` | L2 (Euclidean) distance | `<->` |
| `VecCosineDistance` | Cosine distance | `<=>` |
| `VecInnerProduct` | Negative inner product | `<#>` |

### Equality / Inequality

```go
// Find items with an exact vector match
rows, err := psql.B().Select().From("items").
    Where(psql.VecEqual(psql.F("Embedding"), targetVec)).
    RunQuery(ctx)

// Find items that differ from a vector
rows, err := psql.B().Select().From("items").
    Where(psql.VecNotEqual(psql.F("Embedding"), targetVec)).
    RunQuery(ctx)
```

### Nearest Neighbor Search

Order results by vector distance to find the most similar items:

```go
queryVec := psql.Vector{0.1, 0.2, 0.3, ...}

// Find nearest neighbors by L2 distance
rows, err := psql.B().
    Select("*").
    From("items").
    OrderBy(psql.VecL2Distance(psql.F("Embedding"), queryVec)).
    Limit(10).
    RunQuery(ctx)

// Using VecOrderBy helper
rows, err := psql.B().
    Select("*").
    From("items").
    OrderBy(psql.VecOrderBy(psql.F("Embedding"), queryVec, psql.VectorCosine)).
    Limit(10).
    RunQuery(ctx)
```

### Filtering by Distance

Use distance expressions in WHERE to filter by a threshold:

```go
// Only items within cosine distance 0.5
rows, err := psql.B().Select().From("items").
    Where(psql.Lt(psql.VecCosineDistance(psql.F("Embedding"), queryVec), 0.5)).
    RunQuery(ctx)
```

### Distance as a Column

You can include the distance in your SELECT:

```go
dist := psql.VecCosineDistance(psql.F("Embedding"), queryVec)
rows, err := psql.B().
    Select("*", dist).
    From("items").
    OrderBy(dist).
    Limit(10).
    RunQuery(ctx)
```

## Engine-Specific Behavior

- **PostgreSQL**: Uses pgvector operator syntax (`<->`, `<=>`, `<#>`)
- **CockroachDB**: Uses native vector functions (`vec_l2_distance()`, etc.) and supports pgvector operator syntax
- **Other engines**: Falls back to function call syntax

## Vector Methods

```go
v := psql.Vector{1.0, 2.0, 3.0}

v.String()      // "[1,2,3]"
v.Dimensions()  // 3
```
