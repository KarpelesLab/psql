package psql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"slices"
	"strings"
)

type Set []string

func (s *Set) Set(k string) {
	if !s.Has(k) {
		*s = append(*s, k)
	}
}

func (s *Set) Unset(k string) {
	for n, v := range *s {
		if v == k {
			*s = slices.Delete(*s, n, n)
			return
		}
	}
}

func (s Set) Has(k string) bool {
	for _, v := range s {
		if v == k {
			return true
		}
	}
	return false
}

func (s *Set) Scan(src any) error {
	switch v := src.(type) {
	case string:
		*s = strings.Split(v, ",")
	case []byte:
		*s = strings.Split(string(v), ",")
	case sql.RawBytes:
		*s = strings.Split(string(v), ",")
	default:
		return fmt.Errorf("unsupported input format %T", src)
	}
	return nil
}

func (s Set) Value() (driver.Value, error) {
	v := strings.Join(s, ",")

	return v, nil
}
