package psql_test

import (
	"context"
	"testing"

	"github.com/KarpelesLab/psql"
)

type TestTable1 struct {
	Key  uint64
	Name string `sql:"Name,type=VARCHAR,size=64,null=0"`
}

func TestSQL(t *testing.T) {
	// attempt to connect
	err := psql.Init("/test")
	if err != nil {
		t.Logf("Failed to connect to local MySQL: %s", err)
		t.Skipf("Tests ignored")
		return
	}

	// Drop table if it exists so we start from a clean state
	err = psql.Exec("DROP TABLE IF EXISTS " + psql.QuoteName("Test_Table1"))
	if err != nil {
		t.Errorf("Failed to drop table: %s", err)
	}

	// Insert a value. This will trigger the creation of the table
	v := &TestTable1{Key: 42, Name: "Hello world"}
	err = psql.Insert(context.Background(), v)
	if err != nil {
		t.Fatalf("Failed to insert: %s", err)
	}
}
