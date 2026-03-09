module github.com/KarpelesLab/psql/pgsql

go 1.24.0

require (
	github.com/KarpelesLab/psql v0.0.0
	github.com/jackc/pgx/v5 v5.5.5
)

require (
	github.com/KarpelesLab/pjson v0.1.9 // indirect
	github.com/KarpelesLab/typutil v0.2.12 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/puddle/v2 v2.2.1 // indirect
	golang.org/x/crypto v0.20.0 // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace (
	github.com/KarpelesLab/psql => ../
	github.com/KarpelesLab/psql/sqlite => ../sqlite
)
