# CLAUDE.md - Development Guidelines

## Build & Test Commands
- Build: `make all` - Formats with goimports and builds project
- Dependencies: `make deps` - Gets all dependencies
- Run all tests: `make test` or `go test -v ./...`
- Run single test: `go test -v -run TestName` (e.g., `go test -v -run TestBuilder`)
- Format code: `goimports -w -l .`

## Code Style & Conventions
- Go version: 1.22+ required (supports 1.23 iterators)
- Use custom error handling with `*Error` struct that implements `Unwrap()`
- Error enums defined in package-level vars with `Err` prefix
- Helper functions like `IsNotExist()` for common error patterns
- Package name: `psql` (import as `github.com/portablesql/psql`)
- Use structs with SQL tags for object binding: `sql:",key=PRIMARY"` or `sql:"Name,type=VARCHAR,size=64"`
- Query builder pattern: Start with `psql.B()` followed by methods like `.Select()`, `.From()`, etc.
- Test files use separate `psql_test` package
- SQL functions represented as struct types (`Like`, `FindInSet`, etc.)
- SQL fields referenced with `psql.F("FieldName")`, values with `psql.V("Value")`