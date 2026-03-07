package psql_test

import (
	"testing"

	"github.com/KarpelesLab/psql"
	"github.com/stretchr/testify/assert"
)

func TestEngineString(t *testing.T) {
	assert.Equal(t, "MySQL Engine", psql.EngineMySQL.String())
	assert.Equal(t, "PostgreSQL Engine", psql.EnginePostgreSQL.String())
	assert.Equal(t, "Unknown Engine", psql.EngineUnknown.String())
}

func TestFieldName(t *testing.T) {
	// Simple field
	f := psql.F("name")
	assert.Equal(t, `"name"`, f.EscapeValue())

	// Field with table
	f = psql.F("users", "name")
	assert.Equal(t, `"users"."name"`, f.EscapeValue())

	// Wildcard
	f = psql.F("*")
	assert.Equal(t, `*`, f.EscapeValue())

	// Field with dot notation
	f = psql.F("users.name")
	assert.Equal(t, `"users"."name"`, f.EscapeValue())
}

func TestFieldNamePanics(t *testing.T) {
	assert.Panics(t, func() {
		psql.F("a", "b", "c")
	})
}

func TestSortField(t *testing.T) {
	s := psql.S("name", "ASC")
	assert.NotNil(t, s)

	s = psql.S("name", "DESC")
	assert.NotNil(t, s)

	// Without direction
	s = psql.S("name")
	assert.NotNil(t, s)
}

func TestSortFieldPanics(t *testing.T) {
	assert.Panics(t, func() {
		psql.S()
	})
}

func TestRawValue(t *testing.T) {
	r := psql.Raw("COUNT(*)")
	assert.Equal(t, "COUNT(*)", r.EscapeValue())
}

func TestVValue(t *testing.T) {
	v := psql.V("hello")
	assert.NotNil(t, v)
	// Calling V again on same container should return same
	v2 := psql.V("world")
	assert.NotNil(t, v2)
}

func TestNotValue(t *testing.T) {
	n := &psql.Not{V: "test"}
	assert.Contains(t, n.EscapeValue(), "NOT")
}

func TestComparison(t *testing.T) {
	tests := []struct {
		name     string
		comp     psql.EscapeValueable
		expected string
	}{
		{"Equal", psql.Equal(psql.F("a"), 1), `"a"=1`},
		{"Gt", psql.Gt(psql.F("a"), 1), `"a">1`},
		{"Gte", psql.Gte(psql.F("a"), 1), `"a">=1`},
		{"Lt", psql.Lt(psql.F("a"), 1), `"a"<1`},
		{"Lte", psql.Lte(psql.F("a"), 1), `"a"<=1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.comp.EscapeValue())
		})
	}
}

func TestBetween(t *testing.T) {
	b := psql.Between(psql.F("age"), 18, 65)
	assert.Equal(t, `"age" BETWEEN 18 AND 65`, b.EscapeValue())
}

func TestLike(t *testing.T) {
	l := &psql.Like{Field: psql.F("name"), Like: "test%"}
	assert.Contains(t, l.EscapeValue(), `LIKE 'test%'`)
	assert.Contains(t, l.String(), `LIKE`)
}

func TestFindInSet(t *testing.T) {
	f := &psql.FindInSet{Field: psql.F("tags"), Value: "go"}
	assert.Contains(t, f.EscapeValue(), `FIND_IN_SET(`)
	assert.Contains(t, f.String(), `FIND_IN_SET(`)
}

func TestWhereAND(t *testing.T) {
	w := psql.WhereAND{
		map[string]any{"a": 1},
		map[string]any{"b": 2},
	}
	s := w.EscapeValue()
	assert.Contains(t, s, `"a"=1`)
	assert.Contains(t, s, ` AND `)
	assert.Contains(t, s, `"b"=2`)
	assert.Equal(t, s, w.String())
}

func TestWhereOR(t *testing.T) {
	w := psql.WhereOR{
		map[string]any{"a": 1},
		map[string]any{"b": 2},
	}
	s := w.EscapeValue()
	assert.Contains(t, s, `"a"=1`)
	assert.Contains(t, s, ` OR `)
	assert.Contains(t, s, `"b"=2`)
	assert.Equal(t, s, w.String())
}

func TestHexType(t *testing.T) {
	h := psql.Hex{0xde, 0xad, 0xbe, 0xef}
	v, err := h.Value()
	assert.NoError(t, err)
	assert.Equal(t, "deadbeef", v)

	// Test Scan from string
	var h2 psql.Hex
	err = h2.Scan("deadbeef")
	assert.NoError(t, err)
	assert.Equal(t, psql.Hex{0xde, 0xad, 0xbe, 0xef}, h2)

	// Test Scan from bytes
	var h3 psql.Hex
	err = h3.Scan([]byte("ff00"))
	assert.NoError(t, err)
	assert.Equal(t, psql.Hex{0xff, 0x00}, h3)

	// Test Scan invalid hex
	var h4 psql.Hex
	err = h4.Scan("xyz")
	assert.Error(t, err)

	// Test Scan unsupported type
	var h5 psql.Hex
	err = h5.Scan(42)
	assert.Error(t, err)
}

func TestSetType(t *testing.T) {
	s := psql.Set{"a", "b", "c"}
	assert.True(t, s.Has("a"))
	assert.True(t, s.Has("b"))
	assert.True(t, s.Has("c"))
	assert.False(t, s.Has("d"))

	s.Set("d")
	assert.True(t, s.Has("d"))

	// Set should not add duplicates
	s.Set("d")
	assert.Len(t, s, 4)

	s.Unset("b")
	assert.False(t, s.Has("b"))
	assert.True(t, s.Has("a"))

	v, err := psql.Set{"x", "y"}.Value()
	assert.NoError(t, err)
	assert.Equal(t, "x,y", v)

	// Test Scan
	var s2 psql.Set
	err = s2.Scan("one,two,three")
	assert.NoError(t, err)
	assert.True(t, s2.Has("one"))
	assert.True(t, s2.Has("two"))
	assert.True(t, s2.Has("three"))

	// Test Scan with bytes
	var s3 psql.Set
	err = s3.Scan([]byte("a,b"))
	assert.NoError(t, err)
	assert.True(t, s3.Has("a"))

	// Test Scan with unsupported type
	var s4 psql.Set
	err = s4.Scan(42)
	assert.Error(t, err)
}

func TestFetchOptions(t *testing.T) {
	t.Run("Sort option", func(t *testing.T) {
		opt := psql.Sort(psql.S("name", "ASC"))
		assert.Len(t, opt.Sort, 1)
	})

	t.Run("Limit option", func(t *testing.T) {
		opt := psql.Limit(10)
		assert.Equal(t, 10, opt.LimitCount)
	})

	t.Run("LimitFrom option", func(t *testing.T) {
		opt := psql.LimitFrom(5, 10)
		assert.Equal(t, 5, opt.LimitStart)
		assert.Equal(t, 10, opt.LimitCount)
	})

	t.Run("FetchLock", func(t *testing.T) {
		assert.True(t, psql.FetchLock.Lock)
	})
}
