module github.com/KarpelesLab/psql/mysql

go 1.24.0

require (
	github.com/KarpelesLab/psql v0.0.0
	github.com/go-sql-driver/mysql v1.8.1
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/KarpelesLab/pjson v0.1.9 // indirect
	github.com/KarpelesLab/typutil v0.2.12 // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
)

replace (
	github.com/KarpelesLab/psql => ../
	github.com/KarpelesLab/psql/sqlite => ../sqlite
)
