package psql

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"
)

// mockScanner records the concrete type it received via Scan.
type mockScanner struct {
	gotType string
	gotData []byte
}

func (m *mockScanner) Scan(src any) error {
	if src == nil {
		m.gotType = "<nil>"
		return nil
	}
	m.gotType = fmt.Sprintf("%T", src)
	switch v := src.(type) {
	case []byte:
		m.gotData = make([]byte, len(v))
		copy(m.gotData, v)
	default:
		return fmt.Errorf("unexpected type %T", src)
	}
	return nil
}

func TestScanSetterPassesByteSlice(t *testing.T) {
	// Verify that scanSetter passes []byte (not sql.RawBytes) to Scanner.Scan().
	// Types like uuid.UUID type-switch on []byte and would fail with sql.RawBytes.
	m := &mockScanner{}
	v := reflect.ValueOf(m).Elem() // dereference to the struct value

	raw := sql.RawBytes("hello-world")
	err := scanSetter(v, raw)
	if err != nil {
		t.Fatalf("scanSetter returned error: %v", err)
	}

	if m.gotType != "[]uint8" {
		t.Errorf("Scanner.Scan received %s, want []uint8 ([]byte)", m.gotType)
	}
	if string(m.gotData) != "hello-world" {
		t.Errorf("Scanner.Scan data = %q, want %q", m.gotData, "hello-world")
	}
}

func TestScanSetterNil(t *testing.T) {
	m := &mockScanner{}
	v := reflect.ValueOf(m).Elem()

	err := scanSetter(v, nil)
	if err != nil {
		t.Fatalf("scanSetter returned error: %v", err)
	}
	// nil sql.RawBytes converts to []byte(nil), which wraps as typed nil in any.
	// The scanner receives []uint8 (typed nil), not untyped nil.
	if m.gotType != "[]uint8" {
		t.Errorf("Scanner.Scan received %s for nil input, want []uint8", m.gotType)
	}
}

func TestFindSetterScanner(t *testing.T) {
	// Verify findSetter returns scanSetter for types implementing sql.Scanner.
	typ := reflect.TypeOf((*mockScanner)(nil)) // *mockScanner
	setter := findSetter(typ)
	if setter == nil {
		t.Fatal("findSetter returned nil for sql.Scanner type")
	}

	// Verify it works end-to-end
	m := &mockScanner{}
	v := reflect.ValueOf(m).Elem()
	raw := sql.RawBytes{0xde, 0xad}
	err := setter(v, raw)
	if err != nil {
		t.Fatalf("setter returned error: %v", err)
	}
	if m.gotType != "[]uint8" {
		t.Errorf("got type %s, want []uint8", m.gotType)
	}
}

func TestFindSetterBytes(t *testing.T) {
	typ := reflect.TypeOf([]byte{})
	setter := findSetter(typ)
	if setter == nil {
		t.Fatal("findSetter returned nil for []byte")
	}

	// Verify bytesSetter copies data
	var buf []byte
	v := reflect.ValueOf(&buf).Elem()
	raw := sql.RawBytes{0x01, 0x02, 0x03}
	err := setter(v, raw)
	if err != nil {
		t.Fatalf("setter returned error: %v", err)
	}
	if len(buf) != 3 || buf[0] != 0x01 || buf[1] != 0x02 || buf[2] != 0x03 {
		t.Errorf("got %v, want [1 2 3]", buf)
	}

	// Mutate original — copy should be independent
	raw[0] = 0xff
	if buf[0] != 0x01 {
		t.Error("bytesSetter did not copy data; buffer references driver memory")
	}
}

func TestFindSetterBytesNil(t *testing.T) {
	typ := reflect.TypeOf([]byte{})
	setter := findSetter(typ)

	var buf []byte = []byte{0x01} // non-nil initially
	v := reflect.ValueOf(&buf).Elem()
	err := setter(v, nil)
	if err != nil {
		t.Fatalf("setter returned error: %v", err)
	}
	if buf != nil {
		t.Errorf("expected nil for nil RawBytes, got %v", buf)
	}
}
