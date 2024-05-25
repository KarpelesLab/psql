package psql_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/KarpelesLab/psql"
)

// tests using pgsql

func TestPG(t *testing.T) {
	psql.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))

	// get a cockroach instance
	be, err := psql.LocalTestServer()
	if err != nil {
		t.Skipf("unable to launch cockroach: %s", err)
		return
	}

	ctx := be.Plug(context.Background())

	// Insert a value. This will trigger the creation of the table
	v := &TestTable1{Key: 42, Name: "Hello world"}
	err = psql.Insert(ctx, v)
	if err != nil {
		t.Fatalf("Failed to insert: %s", err)
	}
}
