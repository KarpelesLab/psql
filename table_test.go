package psql_test

import (
	"context"
	"testing"
	"time"

	"github.com/KarpelesLab/psql"
)

type TestTable1 struct {
	Key  uint64
	Name string `sql:"Name,type=VARCHAR,size=64,null=0"`
}

type TestTable1b struct {
	TableName psql.Name `sql:"Test_Table1"`
	Key       uint64
	Name      string `sql:"Name,type=VARCHAR,size=128,null=0"`
	Status    string `sql:"Status,type=ENUM,values='valid,inactive,zombie',default=valid"`
	Created   time.Time
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

	// Instanciate version 1b, should trigger change of Name (size=64 → size=128) and addition of 2 fields
	v2 := &TestTable1b{Key: 43, Name: "Second insert", Status: "valid", Created: time.Now()}
	err = psql.Insert(context.Background(), v2)
	if err != nil {
		t.Fatalf("failed to insert 2: %s", err)
	}
}
