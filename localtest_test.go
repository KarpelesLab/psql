package psql_test

import (
	"context"
	"database/sql"
	"log"
	"testing"

	"github.com/KarpelesLab/psql"
)

func TestLocalTest(t *testing.T) {
	be := getTestBackend(t)
	ctx := be.Plug(context.Background())

	var versionQuery string
	switch be.Engine() {
	case psql.EngineSQLite:
		versionQuery = "SELECT sqlite_version()"
	default:
		versionQuery = "SELECT VERSION()"
	}

	err := psql.Q(versionQuery).Each(ctx, func(row *sql.Rows) error {
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
