package psql

import (
	"errors"
	"fmt"
)

type Error struct {
	Query string
	Err   error
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) Error() string {
	return fmt.Sprintf("While running %s: %s", e.Query, e.Err)
}

var ErrNotNillable = errors.New("field is nil but cannot be nil")
