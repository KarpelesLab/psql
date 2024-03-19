package psql_test

import (
	"context"
	"log/slog"
	"os"
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
	Flags     string    `sql:"Flags,type=SET,null=0,values='red,green,blue',default=red"`
	Created   time.Time
	NameKey   psql.Key `sql:",type=UNIQUE,fields=Name"`
	StatusKey psql.Key `sql:",fields=Status"`
}

func TestSQL(t *testing.T) {
	psql.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, nil)))
	//os.Stderr, "psql: ", log.LstdFlags|log.Lmsgprefix))

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
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": []any{42}})
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
	if v3.Key != 43 {
		t.Errorf("Fetch 43: bad id")
	}
	if v3.Name != "Second insert" {
		t.Errorf("Fetch 43: bad name")
	}
	if v3.Status != "valid" {
		t.Errorf("Fetch 43: bad status")
	}
	// fetch 43 by name (like)
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Name": &psql.Like{Like: "Second%"}})
	if err != nil {
		t.Fatalf("failed to fetch 43: %s", err)
	}
	if v3.Key != 43 {
		t.Errorf("Fetch 43: bad id")
	}
	// fetch 43 by comparison
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": psql.Gt(nil, 42)})
	if err != nil {
		t.Fatalf("failed to fetch 43: %s", err)
	}
	if v3.Key != 43 {
		t.Errorf("Fetch 43: bad id")
	}

	// Try to fetch 44 → not found error
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": 44})
	if !psql.IsNotExist(err) {
		t.Errorf("Fetch 44: should be not found, but error was %v", err)
	}

	lst, err := psql.Fetch[TestTable1b](context.Background(), nil)
	if err != nil {
		t.Fatalf("failed to fetch all: %s", err)
	}
	if len(lst) != 2 {
		t.Fatalf("expected 2 results, got %d", len(lst))
	}

	// Re-fetch 42
	err = psql.FetchOne(context.Background(), v3, map[string]any{"Key": 42})
	if err != nil {
		t.Fatalf("failed to fetch 42: %s", err)
	}

	if psql.HasChanged(v3) {
		t.Errorf("Reports changes despite no changes yet")
	}

	// updte a value into 42
	v3.Name = "Updated name"

	if !psql.HasChanged(v3) {
		t.Errorf("Update 42 does not report changes")
	}

	err = psql.Update(context.Background(), v3)
	if err != nil {
		t.Fatalf("failed to update 42: %s", err)
	}

	// shouldn't be changed anymore
	if psql.HasChanged(v3) {
		t.Errorf("Reports changes despite no changes yet")
	}

	var v4 = &TestTable1b{}

	// Re-fetch 42
	err = psql.FetchOne(context.Background(), v4, map[string]any{"Key": 42})
	if err != nil {
		t.Fatalf("failed to fetch 42: %s", err)
	}

	if v4.Name != "Updated name" {
		t.Errorf("failed to update name for 42")
	}
}
