package psql_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/KarpelesLab/psql"
)

// tests using pgsql

type TestPgTable1 struct {
	Key     uint64   `sql:",key=PRIMARY"`
	Name    string   `sql:"Name,type=VARCHAR,size=64,null=0"`
	NameKey psql.Key `sql:",type=UNIQUE,fields=Name"`
}

func TestPG(t *testing.T) {
	psql.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	be := getTestBackend(t)
	if be.Engine() != psql.EnginePostgreSQL {
		t.Skip("Test only applicable for PostgreSQL")
	}

	ctx := be.Plug(context.Background())

	// Insert a value. This will trigger the creation of the table
	v := &TestPgTable1{Key: 42, Name: "Hello world"}
	err := psql.Insert(ctx, v)
	if err != nil {
		t.Fatalf("Failed to insert: %s", err)
	}
}
