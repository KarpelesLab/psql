package psql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Vector represents a vector of float32 values, compatible with PostgreSQL pgvector
// and CockroachDB native vector types.
//
// Usage in struct tags:
//
//	Embedding Vector `sql:",type=VECTOR,size=3"` // 3-dimensional vector
type Vector []float32

var _ sql.Scanner = (*Vector)(nil)
var _ driver.Valuer = (*Vector)(nil)

// String returns the vector in PostgreSQL/CockroachDB format: [1,2,3]
func (v Vector) String() string {
	if v == nil {
		return ""
	}
	b := &strings.Builder{}
	b.WriteByte('[')
	for i, val := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(val), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// Scan implements sql.Scanner for reading vector values from the database.
// Supports PostgreSQL pgvector format [1,2,3] and string representations.
func (v *Vector) Scan(src any) error {
	if src == nil {
		*v = nil
		return nil
	}

	var s string
	switch val := src.(type) {
	case string:
		s = val
	case []byte:
		s = string(val)
	default:
		// Handle types that are convertible to []byte or string (e.g. sql.RawBytes)
		rv := reflect.ValueOf(src)
		if rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
			s = string(rv.Bytes())
		} else if rv.Kind() == reflect.String {
			s = rv.String()
		} else {
			return fmt.Errorf("psql: cannot scan %T into Vector", src)
		}
	}

	s = strings.TrimSpace(s)
	if s == "" {
		*v = nil
		return nil
	}

	// Strip brackets
	if len(s) >= 2 && s[0] == '[' && s[len(s)-1] == ']' {
		s = s[1 : len(s)-1]
	}

	if s == "" {
		*v = Vector{}
		return nil
	}

	parts := strings.Split(s, ",")
	result := make(Vector, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return fmt.Errorf("psql: invalid vector component %q: %w", p, err)
		}
		result[i] = float32(f)
	}
	*v = result
	return nil
}

// Value implements driver.Valuer for writing vector values to the database.
func (v Vector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	return v.String(), nil
}

// Dimensions returns the number of dimensions in the vector.
func (v Vector) Dimensions() int {
	return len(v)
}
