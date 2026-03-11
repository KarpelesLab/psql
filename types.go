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

// Raw creates a raw SQL expression that is injected verbatim into queries without
// escaping. Use carefully to avoid SQL injection:
//
//	psql.B().Select(psql.Raw("COUNT(*)")).From("users")
func Raw(s string) EscapeValueable {
	return &rawValue{s}
}

// Not negates a condition. Wraps any value to produce IS NOT NULL, !=, NOT LIKE, etc.
//
//	&psql.Not{V: nil}                          // IS NOT NULL
//	&psql.Not{V: psql.Equal(psql.F("a"), "b")} // a != b
type Not struct {
	V any
}

func (n *Not) EscapeValue() string {
	res := "NOT ("
	res += Escape(n.V)
	res += ")"
	return res
}

func (n *Not) escapeValueCtx(ctx *renderContext) string {
	res := "NOT ("
	res += escapeCtx(ctx, n.V)
	res += ")"
	return res
}
