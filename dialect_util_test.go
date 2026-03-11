package psql_test

import (
	"testing"

	"github.com/portablesql/psql"
	"github.com/stretchr/testify/assert"
)

func TestDefaultExportArgNilSlice(t *testing.T) {
	// A typed nil []byte passed as any is not == nil at the interface level.
	// DefaultExportArg must detect it and return untyped nil.
	var b []byte // nil slice
	result := psql.DefaultExportArg(b)
	assert.Nil(t, result, "nil []byte should export as nil")
}

func TestDefaultExportArgNilMap(t *testing.T) {
	var m map[string]string // nil map
	result := psql.DefaultExportArg(m)
	assert.Nil(t, result, "nil map should export as nil")
}

func TestDefaultExportArgNonNilSlice(t *testing.T) {
	b := []byte{0x01, 0x02}
	result := psql.DefaultExportArg(b)
	assert.Equal(t, b, result, "non-nil []byte should pass through")
}

func TestDefaultExportArgNilPointer(t *testing.T) {
	var p *string
	result := psql.DefaultExportArg(p)
	assert.Nil(t, result, "nil pointer should export as nil")
}

func TestDefaultExportArgNil(t *testing.T) {
	result := psql.DefaultExportArg(nil)
	assert.Nil(t, result, "untyped nil should export as nil")
}
