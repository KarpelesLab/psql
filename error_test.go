package psql_test

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/portablesql/psql"
	"github.com/stretchr/testify/assert"
)

func TestErrorType(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	e := &psql.Error{Query: "SELECT 1", Err: inner}

	assert.Contains(t, e.Error(), "SELECT 1")
	assert.Contains(t, e.Error(), "connection refused")
	assert.Equal(t, inner, e.Unwrap())
}

func TestErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("base error")
	e := &psql.Error{Query: "SELECT 1", Err: inner}
	assert.True(t, errors.Is(e, inner))
}

func TestIsNotExistWithOsError(t *testing.T) {
	assert.True(t, psql.IsNotExist(os.ErrNotExist))
	assert.False(t, psql.IsNotExist(fmt.Errorf("some other error")))
}

func TestErrorNumber(t *testing.T) {
	// With nil error
	assert.Equal(t, uint16(0), psql.ErrorNumber(nil))

	// With non-MySQL error
	assert.Equal(t, uint16(0xffff), psql.ErrorNumber(fmt.Errorf("generic error")))

	// With wrapped error
	inner := fmt.Errorf("base error")
	wrapped := &psql.Error{Query: "SELECT 1", Err: inner}
	assert.Equal(t, uint16(0xffff), psql.ErrorNumber(wrapped))
}

func TestSentinelErrors(t *testing.T) {
	assert.NotNil(t, psql.ErrNotReady)
	assert.NotNil(t, psql.ErrNotNillable)
	assert.NotNil(t, psql.ErrTxAlreadyProcessed)
	assert.NotNil(t, psql.ErrDeleteBadAssert)
	assert.NotNil(t, psql.ErrBreakLoop)
}
