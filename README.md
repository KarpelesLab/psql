[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/psql.svg)](https://pkg.go.dev/github.com/KarpelesLab/psql)

# psql

Platform SQL code, including object load/save & query builder.

This works in some ways similar to `gorm` but with focus on supporting and using modern Go syntax & features.

## Object binding

After defining a structure, you can use it to load/save data from database.

```go
type Table1 struct {
    Key uint64 `sql:",key=PRIMARY"`
    Name string `sql:"Name,type=VARCHAR,size=64"`
}

// ...

obj, err := psql.Get[Table1](context.Background(), map[string]any{"Key": 42}) // this fetches entry with Key=42
```

## go 1.23

New go 1.23 iterators can be used

```go
res, err := psql.Iter[Table1](context.Background(), map[string]any{"Type": "A"}) // this fetches entries with Type=A
if err != nil {
    return err
}
for v := range res {
    // v is of type *Table1
}
```

## Query builder
