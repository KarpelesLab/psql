package psql

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

func findSetter(t reflect.Type) func(v reflect.Value, from sql.RawBytes) error {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if reflect.PtrTo(t).Implements(reflect.TypeOf((*sql.Scanner)(nil)).Elem()) {
		return scanSetter
	}

	// most specific types
	switch t {
	case reflect.TypeOf(time.Time{}):
		return timeSetter
	}

	// fallbacks
	switch t.Kind() {
	case reflect.Bool:
		return boolSetter
	case reflect.String:
		return stringSetter
	case reflect.Int32:
		return int32Setter
	case reflect.Int64, reflect.Int:
		return int64Setter
	case reflect.Uint32:
		return uint32Setter
	case reflect.Uint64, reflect.Uint:
		return uint64Setter
	case reflect.Float32:
		return float32Setter
	case reflect.Float64:
		return float64Setter
	default:
		panic(fmt.Sprintf("no setter for type %s", t))
	}
}

func scanSetter(v reflect.Value, from sql.RawBytes) error {
	// v implements Scanner, so we should call v.Scan(from)
	v = v.Addr() // use the pointer version

	intf := v.Interface().(sql.Scanner)

	return intf.Scan(from)
}

func boolSetter(v reflect.Value, from sql.RawBytes) error {
	// expect "from" to be 1 or 0
	switch string(from) {
	case "1", "true", "TRUE", "True":
		v.SetBool(true)
		return nil
	case "0", "false", "FALSE", "False":
		v.SetBool(false)
		return nil
	default:
		return errors.New("invalid bool value")
	}
}

func stringSetter(v reflect.Value, from sql.RawBytes) error {
	v.SetString(string(from))
	return nil
}

func int32Setter(v reflect.Value, from sql.RawBytes) error {
	n, err := strconv.ParseInt(string(from), 10, 32)
	if err != nil {
		return err
	}
	v.SetInt(n)
	return nil
}

func uint32Setter(v reflect.Value, from sql.RawBytes) error {
	n, err := strconv.ParseUint(string(from), 10, 32)
	if err != nil {
		return err
	}
	v.SetUint(n)
	return nil
}

func int64Setter(v reflect.Value, from sql.RawBytes) error {
	n, err := strconv.ParseInt(string(from), 10, 64)
	if err != nil {
		return err
	}
	v.SetInt(n)
	return nil
}

func uint64Setter(v reflect.Value, from sql.RawBytes) error {
	n, err := strconv.ParseUint(string(from), 10, 64)
	if err != nil {
		return err
	}
	v.SetUint(n)
	return nil
}

func float32Setter(v reflect.Value, from sql.RawBytes) error {
	n, err := strconv.ParseFloat(string(from), 32)
	if err != nil {
		return err
	}
	v.SetFloat(n)
	return nil
}

func float64Setter(v reflect.Value, from sql.RawBytes) error {
	n, err := strconv.ParseFloat(string(from), 64)
	if err != nil {
		return err
	}
	v.SetFloat(n)
	return nil
}

func timeSetter(v reflect.Value, from sql.RawBytes) error {
	// parse date
	// RFC3339Nano = "2006-01-02T15:04:05.999999999Z07:00"
	if bytes.IndexByte(from, 'T') != -1 {
		// this is a RFC3339 date
		t, err := time.Parse(time.RFC3339Nano, string(from))
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(t))
		return nil
	}

	const base = "2006-01-02 15:04:05.999999"
	const zero = "0000-00-00 00:00:00.000000"
	switch len(from) {
	case 10, 19, 21, 22, 23, 24, 25, 26: // up to "YYYY-MM-DD HH:MM:SS.MMMMMM"
		if string(from) == zero[:len(from)] {
			// zero time
			v.Set(reflect.ValueOf(time.Time{}))
			return nil
		}

		// In the absence of a time zone indicator, Parse returns a time in UTC.
		t, err := time.Parse(base[:len(from)], string(from))
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(t))
		return nil
	}
	return fmt.Errorf("failed to parse time: %s", from)
}
