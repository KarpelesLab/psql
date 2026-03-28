package psql

import (
	"database/sql"
	"encoding/json"
	"reflect"
	"testing"
)

// === JSON setter tests ===

func TestJSONSetterMap(t *testing.T) {
	setter := makeJSONSetter(reflect.TypeOf(map[string]any{}))

	var m map[string]any
	v := reflect.ValueOf(&m).Elem()

	raw := sql.RawBytes(`{"name":"alice","age":30}`)
	if err := setter(v, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["name"] != "alice" {
		t.Errorf("got name=%v, want alice", m["name"])
	}
	if m["age"] != float64(30) {
		t.Errorf("got age=%v, want 30", m["age"])
	}
}

func TestJSONSetterSlice(t *testing.T) {
	setter := makeJSONSetter(reflect.TypeOf([]string{}))

	var s []string
	v := reflect.ValueOf(&s).Elem()

	raw := sql.RawBytes(`["a","b","c"]`)
	if err := setter(v, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != 3 || s[0] != "a" || s[1] != "b" || s[2] != "c" {
		t.Errorf("got %v, want [a b c]", s)
	}
}

func TestJSONSetterStruct(t *testing.T) {
	type inner struct {
		X int    `json:"x"`
		Y string `json:"y"`
	}
	setter := makeJSONSetter(reflect.TypeOf(inner{}))

	var obj inner
	v := reflect.ValueOf(&obj).Elem()

	raw := sql.RawBytes(`{"x":42,"y":"hello"}`)
	if err := setter(v, raw); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obj.X != 42 || obj.Y != "hello" {
		t.Errorf("got %+v, want {X:42 Y:hello}", obj)
	}
}

func TestJSONSetterEmpty(t *testing.T) {
	setter := makeJSONSetter(reflect.TypeOf(map[string]any{}))

	m := map[string]any{"leftover": true}
	v := reflect.ValueOf(&m).Elem()

	// Empty input should zero the value
	if err := setter(v, sql.RawBytes{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m != nil {
		t.Errorf("got %v, want nil", m)
	}
}

func TestJSONSetterInvalid(t *testing.T) {
	setter := makeJSONSetter(reflect.TypeOf(map[string]any{}))

	var m map[string]any
	v := reflect.ValueOf(&m).Elem()

	raw := sql.RawBytes(`not json`)
	err := setter(v, raw)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// === JSON export tests ===

func TestJSONExportMap(t *testing.T) {
	f := &StructField{Attrs: map[string]string{"format": "json"}}
	in := map[string]any{"key": "value", "n": 1}
	out := EngineMySQL.export(in, f)

	s, ok := out.(string)
	if !ok {
		t.Fatalf("expected string, got %T", out)
	}
	var decoded map[string]any
	if err := json.Unmarshal([]byte(s), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if decoded["key"] != "value" {
		t.Errorf("got key=%v, want value", decoded["key"])
	}
}

func TestJSONExportSlice(t *testing.T) {
	f := &StructField{Attrs: map[string]string{"format": "json"}}
	in := []string{"a", "b", "c"}
	out := EngineMySQL.export(in, f)

	s, ok := out.(string)
	if !ok {
		t.Fatalf("expected string, got %T", out)
	}
	if s != `["a","b","c"]` {
		t.Errorf("got %s, want %s", s, `["a","b","c"]`)
	}
}

func TestJSONExportNil(t *testing.T) {
	f := &StructField{Attrs: map[string]string{"format": "json"}}
	out := EngineMySQL.export(nil, f)
	if out != nil {
		t.Errorf("expected nil, got %v", out)
	}
}

func TestExportWithoutFormat(t *testing.T) {
	// Without format=json, export should pass through to dialect
	f := &StructField{Attrs: map[string]string{"type": "TEXT"}}
	out := EngineMySQL.export("hello", f)
	if out != "hello" {
		t.Errorf("expected hello, got %v", out)
	}
}

// === Table integration: verify format=json fields get JSON setter ===

type jsonTestRow struct {
	Name    `sql:"json_test"`
	ID      uint64         `sql:",key=PRIMARY"`
	Data    map[string]any `sql:"data,type=TEXT,format=json"`
	Tags    []string       `sql:"tags,type=TEXT,format=json"`
	Regular string         `sql:"regular,type=VARCHAR,size=255"`
}

func TestTableJSONFieldSetter(t *testing.T) {
	tbl := Table[jsonTestRow]()

	// Find the "data" field
	dataField, ok := tbl.fldcol["data"]
	if !ok {
		t.Fatal("data field not found")
	}
	if dataField.Attrs["format"] != "json" {
		t.Error("data field should have format=json attr")
	}

	// Verify the setter works with JSON
	var m map[string]any
	v := reflect.ValueOf(&m).Elem()
	err := dataField.setter(v, sql.RawBytes(`{"hello":"world"}`))
	if err != nil {
		t.Fatalf("setter error: %v", err)
	}
	if m["hello"] != "world" {
		t.Errorf("got %v, want world", m["hello"])
	}

	// Find the "tags" field
	tagsField, ok := tbl.fldcol["tags"]
	if !ok {
		t.Fatal("tags field not found")
	}

	var s []string
	v = reflect.ValueOf(&s).Elem()
	err = tagsField.setter(v, sql.RawBytes(`["go","sql"]`))
	if err != nil {
		t.Fatalf("setter error: %v", err)
	}
	if len(s) != 2 || s[0] != "go" || s[1] != "sql" {
		t.Errorf("got %v, want [go sql]", s)
	}

	// Regular field should NOT have a JSON setter (verify it's a normal string setter)
	regularField, ok := tbl.fldcol["regular"]
	if !ok {
		t.Fatal("regular field not found")
	}
	var str string
	v = reflect.ValueOf(&str).Elem()
	err = regularField.setter(v, sql.RawBytes("plain text"))
	if err != nil {
		t.Fatalf("setter error: %v", err)
	}
	if str != "plain text" {
		t.Errorf("got %q, want %q", str, "plain text")
	}
}

// === Roundtrip test: export then import ===

func TestJSONRoundtrip(t *testing.T) {
	f := &StructField{Attrs: map[string]string{"format": "json"}}
	setter := makeJSONSetter(reflect.TypeOf(map[string]any{}))

	original := map[string]any{"key": "value", "nested": map[string]any{"a": float64(1)}}

	// Export
	exported := EngineMySQL.export(original, f)
	jsonStr, ok := exported.(string)
	if !ok {
		t.Fatalf("expected string, got %T", exported)
	}

	// Import
	var restored map[string]any
	v := reflect.ValueOf(&restored).Elem()
	if err := setter(v, sql.RawBytes(jsonStr)); err != nil {
		t.Fatalf("setter error: %v", err)
	}

	if restored["key"] != "value" {
		t.Errorf("roundtrip failed: got key=%v", restored["key"])
	}
	nested, ok := restored["nested"].(map[string]any)
	if !ok {
		t.Fatalf("nested is %T, want map", restored["nested"])
	}
	if nested["a"] != float64(1) {
		t.Errorf("roundtrip failed: got nested.a=%v", nested["a"])
	}
}
