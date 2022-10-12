package psql

import (
	"database/sql/driver"
	"strings"
)

type typeContainer struct {
	v driver.Value
}

func (t *typeContainer) Value() (driver.Value, error) {
	return t.v, nil
}

// V ensures a given value is a value and cannot be interpreted as something else
func V(v driver.Value) driver.Value {
	switch v.(type) {
	case *typeContainer:
		return v
	default:
		return &typeContainer{v}
	}
}

type Raw struct {
	V string
}

func (r *Raw) EscapeValue() string {
	return r.V
}

type Set struct {
	K, V any
}

func (s *Set) EscapeValue() string {
	b := &strings.Builder{}
	switch v := s.K.(type) {
	case string:
		b.WriteString(Escape(FieldName(v)))
	default:
		b.WriteString(Escape(s.K))
	}
	b.WriteByte('=')
	b.WriteString(Escape(s.V))
	return b.String()
}
