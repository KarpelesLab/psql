package psql_test

import (
	"context"
	"testing"
	"time"

	"github.com/KarpelesLab/psql"
)

type TestTable1 struct {
	Key     uint64   `sql:",key=PRIMARY"`
	Name    string   `sql:"Name,type=VARCHAR,size=64,null=0"`
	NameKey psql.Key `sql:",type=UNIQUE,fields=Name"`
}

type TestTable1b struct {
	TableName psql.Name `sql:"Test_Table1"`
	Key       uint64    `sql:",key=PRIMARY"`
	Name      string    `sql:"Name,type=VARCHAR,size=128,null=0"`
	Status    string    `sql:"Status,type=ENUM,null=0,values='valid,inactive,zombie,new',default=new"`
	Created   time.Time
	NameKey   psql.Key `sql:",type=UNIQUE,fields=Name"`
	StatusKey psql.Key `sql:",fields=Status"`
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
	err = psql.Exec(psql.Q("DROP TABLE IF EXISTS " + psql.QuoteName("Test_Table1")))
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

	// test values
	var v3 = &TestTable1b{}

	// we don't allow passing a pointer there anymore
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": 42})
	if err != nil {
		t.Fatalf("failed to fetch 42: %s", err)
	}
	if v3.Name != "Hello world" {
		t.Errorf("Fetch 42: bad name")
	}
	if v3.Status != "new" {
		t.Errorf("Fetch 42: bad status")
	}

	// fetch 43
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": 43})
	if err != nil {
		t.Fatalf("failed to fetch 43: %s", err)
	}
	if v3.Name != "Second insert" {
		t.Errorf("Fetch 43: bad name")
	}
	if v3.Status != "valid" {
		t.Errorf("Fetch 43: bad status")
	}

	// Try to fetch 44 → not found error
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": 44})
	if !psql.IsNotExist(err) {
		t.Errorf("Fetch 44: should be not found, but error was %v", err)
	}

	// Re-fetch 42
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": 42})
	if err != nil {
		t.Fatalf("failed to fetch 42: %s", err)
	}

	if !psql.HasChanged(v3) {
		t.Errorf("Reports changes despite no changes yet")
	}

	// updte a value into 42
	v3.Name = "Updated name"

	if !psql.HasChanged(v3) {
		t.Errorf("Update 42 does not report changes")
	}
}
