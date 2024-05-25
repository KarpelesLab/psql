package psql_test

import (
	"context"
	"database/sql"
	"log"
	"testing"

	"github.com/KarpelesLab/psql"
)

func TestLocalTest(t *testing.T) {
	// this tests if we actually run a server
	be, err := psql.LocalTestServer()
	if err != nil {
		t.Errorf("unable to launch cockroach: %s", err)
		return
	}

	ctx := be.Plug(context.Background())

	err = psql.Q("SELECT VERSION()").Each(ctx, func(row *sql.Rows) error {
		var version string
		if err := row.Scan(&version); err != nil {
			return err
		}

		log.Printf("version = %s", version)
		return nil
	})

	if err != nil {
		t.Errorf("failed to get version: %s", err)
	}
}
