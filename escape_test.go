package psql_test

import (
	"testing"
	"time"

	"github.com/portablesql/psql"
	"github.com/stretchr/testify/assert"
)

func TestEscapeString(t *testing.T) {
	assert.Equal(t, "'hello'", psql.Escape("hello"))
	assert.Equal(t, "'it''s'", psql.Escape("it's"))
	assert.Equal(t, "''", psql.Escape(""))
}

func TestEscapeNil(t *testing.T) {
	assert.Equal(t, "NULL", psql.Escape(nil))
}

func TestEscapeInt(t *testing.T) {
	assert.Equal(t, "42", psql.Escape(int64(42)))
	assert.Equal(t, "-1", psql.Escape(int64(-1)))
	assert.Equal(t, "0", psql.Escape(int64(0)))
}

func TestEscapeFloat(t *testing.T) {
	assert.Equal(t, "3.14", psql.Escape(float64(3.14)))
	assert.Equal(t, "0", psql.Escape(float64(0)))
}

func TestEscapeBool(t *testing.T) {
	assert.Equal(t, "TRUE", psql.Escape(true))
	assert.Equal(t, "FALSE", psql.Escape(false))
}

func TestEscapeBytes(t *testing.T) {
	assert.Equal(t, "x'ff00beef'", psql.Escape([]byte{0xff, 0x00, 0xbe, 0xef}))
	assert.Equal(t, "x''", psql.Escape([]byte{}))
	assert.Equal(t, "NULL", psql.Escape([]byte(nil)))
}

func TestEscapeTime(t *testing.T) {
	tm := time.Date(2024, 1, 15, 12, 30, 45, 0, time.UTC)
	assert.Equal(t, "'2024-01-15 12:30:45'", psql.Escape(tm))

	zero := time.Time{}
	assert.Equal(t, "'0000-00-00 00:00:00.000000'", psql.Escape(zero))
}

func TestEscapeCustomTypes(t *testing.T) {
	// Test with typed int
	type MyInt int
	assert.Equal(t, "42", psql.Escape(MyInt(42)))

	// Test with typed string
	type MyStr string
	assert.Equal(t, "'hello'", psql.Escape(MyStr("hello")))

	// Test with typed bool
	type MyBool bool
	assert.Equal(t, "TRUE", psql.Escape(MyBool(true)))
}

func TestEscapePointer(t *testing.T) {
	s := "hello"
	assert.Equal(t, "'hello'", psql.Escape(&s))

	var nilPtr *string
	assert.Equal(t, "NULL", psql.Escape(nilPtr))
}

func TestQuoteName(t *testing.T) {
	assert.Equal(t, `"users"`, psql.QuoteName("users"))
	assert.Equal(t, `"has""quote"`, psql.QuoteName(`has"quote`))
}
