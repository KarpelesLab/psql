package psql

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"strconv"
)

// intStr is a helper to convert int to string.
func intStr(v int) string {
	return strconv.Itoa(v)
}

// DefaultExportArg handles shared export logic for types common to all engines.
// Submodule dialects should call this as a fallback from their ExportArg implementations.
func DefaultExportArg(v any) any {
	return defaultExportArg(v)
}

// defaultExportArg handles shared export logic for types common to all engines.
func defaultExportArg(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case fmt.Stringer:
		return val.String()
	case driver.Valuer:
		return v
	default:
		rv := reflect.ValueOf(v)
		if rv.Type().Kind() == reflect.Ptr {
			if rv.IsNil() {
				return nil
			}
			return defaultExportArg(rv.Elem().Interface())
		}
		return v
	}
}
