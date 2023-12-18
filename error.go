package psql

import (
	"errors"
	"fmt"
	"os"

	"github.com/go-sql-driver/mysql"
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

// IsNotExist returns true if the error is relative to a table not existing.
//
// See: https://mariadb.com/kb/en/mariadb-error-codes/
//
// Example:
// Error 1146: Table 'test.Test_Table1' doesn't exist
func IsNotExist(err error) bool {
	switch ErrorNumber(err) {
	case 1008: // Can't drop database '%s'; database doesn't exist
	case 1029: // View '%s' doesn't exist for '%s'
	case 1049: // Unknown database '%s'
	case 1051: // Unknown table '%s'
	case 1054: // Unknown column '%s' in '%s'
	case 1072: // Key column '%s' doesn't exist in table
	case 1091: // Can't DROP '%s'; check that column/key exists
	case 1109: // Unknown table '%s' in %s
	case 1141: // There is no such grant defined for user '%s' on host '%s'
	case 1146: // Table '%s.%s' doesn't exist
	case 1147: // There is no such grant defined for user '%s' on host '%s' on table '%s'
	case 1176: // Key '%s' doesn't exist in table '%s'
	case 1305: // %s %s does not exist
	case 1360: // Trigger does not exist
	case 1431: // The foreign data source you are trying to reference does not exist. Data source error: %s
	case 1449: // The user specified as a definer ('%s'@'%s') does not exist
	case 1477: // The foreign server name you are trying to reference does not exist. Data source error: %s
	case 1539: // Unknown event '%s'
	case 1630: // FUNCTION %s does not exist. Check the 'Function Name Parsing and Resolution' section in the Reference Manual
	case 1749: // partition '%s' doesn't exist
	case 1974: // Can't drop user '%-.64s'@'%-.64s'; it doesn't exist
	case 1976: // Can't drop role '%-.64s'; it doesn't exist
	case 4031: // Referenced trigger '%s' for the given action time and event type does not exist
	case 4162: // Operator does not exists: '%-.128s'
	default:
		// in some cases we replace error with fs.ErrNotExist, check for that too
		return os.IsNotExist(err)
	}
	return true
}

func ErrorNumber(err error) uint16 {
	for {
		if err == nil {
			// no error
			return 0
		}
		switch e := err.(type) {
		case *mysql.MySQLError:
			return e.Number
		case interface{ Unwrap() error }:
			err = e.Unwrap()
		default:
			// unknown error type, 0xffff can be differenciated from 0
			return 0xffff
		}
	}
}

var (
	ErrNotReady           = errors.New("database is not ready (no connection is available)")
	ErrNotNillable        = errors.New("field is nil but cannot be nil")
	ErrTxAlreadyProcessed = errors.New("transaction has already been committed or rollbacked")
	ErrDeleteBadAssert    = errors.New("delete operation failed assertion")
)
