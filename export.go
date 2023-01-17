package psql

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"time"
)

// export transforms various known types to types easier to handle for MySQL
func export(in any) any {
	if in == nil {
		return nil
	}
	switch v := in.(type) {
	case time.Time:
		if v.IsZero() {
			return "0000-00-00 00:00:00.000000"
		}
		return v.UTC().Format("2006-01-02 15:04:05.999999")
	case fmt.Stringer:
		return v.String()
	case driver.Valuer:
		return v
	default:
		val := reflect.ValueOf(in)
		if val.Type().Kind() == reflect.Ptr {
			// retry but dererence value
			return export(val.Elem().Interface())
		}
		return in
	}
}
