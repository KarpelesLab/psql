[![Go Reference](https://pkg.go.dev/badge/github.com/KarpelesLab/psql.svg)](https://pkg.go.dev/github.com/KarpelesLab/psql)

# psql

Platform SQL code, including object load/save & query builder.

## Object binding

After defining a structure, you can use it to load/save data from database.

```go
type Table1 struct {
	Key uint64
	Name string `sql:"Name,type=VARCHAR,size=64"`
}

// ...

var obj *Table1
err := psql.FetchOne(nil, &obj, map[string]any{"Key": 42}) // this fetches entry with Key=42
```

## Query builder
