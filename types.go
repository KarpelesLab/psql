package psql

import (
	"database/sql/driver"
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

type rawValue struct {
	V string
}

func (r *rawValue) EscapeValue() string {
	return r.V
}

func (r *rawValue) sortEscapeValue() string {
	return r.V
}

func Raw(s string) EscapeValueable {
	return &rawValue{s}
}

type Not struct {
	V any
}

func (n *Not) EscapeValue() string {
	res := "NOT ("
	res += Escape(n.V)
	res += ")"
	return res
}
