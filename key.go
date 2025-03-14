package psql

import (
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

var keyType = reflect.TypeFor[Key]()

// Name allows specifying the table name when associating a table with a struct
//
// For example:
// type X struct {
// KeyName psql.Key `sql:",type=UNIQUE,fields='A,B'"`
// ...
// }
type Key struct {
	st *rowState
}

func (k *Key) state() *rowState {
	if k.st == nil {
		k.st = &rowState{}
	}
	return k.st
}

const (
	keyPrimary = iota + 1
	keyUnique
	keyIndex
	keyFulltext
	keySpatial
)

type structKey struct {
	index  int
	name   string
	key    string // key name, can be != name
	typ    int
	attrs  map[string]string
	fields []string
}

func (k *structKey) loadAttrs(attrs map[string]string) {
	k.typ = keyIndex // default value
	if t, ok := attrs["type"]; ok {
		switch strings.ToUpper(t) {
		case "PRIMARY":
			k.typ = keyPrimary
		case "UNIQUE":
			k.typ = keyUnique
		case "INDEX":
			k.typ = keyIndex
		case "FULLTEXT":
			k.typ = keyFulltext
		case "SPATIAL":
			k.typ = keySpatial
		default:
			slog.Warn(fmt.Sprintf("[psql] Unsupported index key type %s assumed as INDEX", t), "event", "psql:key:badkey", "psql.index", k.name)
		}
	} else if k.key == "PRIMARY" {
		k.typ = keyPrimary
	}
	k.attrs = attrs
	k.fields = strings.Split(attrs["fields"], ",")
}

func (k *structKey) loadKeyName(kn string) {
	switch {
	case kn == "PRIMARY":
		k.typ = keyPrimary
	case strings.HasPrefix(kn, "UNIQUE:"):
		kn = strings.TrimPrefix(kn, "UNIQUE:")
		k.typ = keyUnique
	}
	k.name = kn
	k.key = kn
}

func (k *structKey) keyname() string {
	if k.typ == keyPrimary {
		return "PRIMARY"
	}
	return k.key
}

func (k *structKey) sqlKeyName() string {
	if k.typ == keyPrimary {
		return "PRIMARY KEY"
	}
	return "INDEX " + QuoteName(k.key)
}

func (k *structKey) defString(e Engine) string {
	s := &strings.Builder{}

	switch k.typ {
	case keyPrimary:
		// PRIMARY KEY [index_type] (key_part,...) [index_option] ...
		// Primary keys don't have names
		s.WriteString("PRIMARY KEY ")
	case keyUnique:
		// UNIQUE [INDEX | KEY] [index_name] [index_type] (key_part,...) [index_option] ...
		s.WriteString("UNIQUE INDEX ")
		s.WriteString(QuoteName(k.key))
	case keyIndex:
		// {INDEX | KEY} [index_name] [index_type] (key_part,...) [index_option] ...
		s.WriteString("INDEX ")
		s.WriteString(QuoteName(k.key))
	case keyFulltext:
		// {FULLTEXT | SPATIAL} [INDEX | KEY] [index_name] (key_part,...) [index_option] ...
		s.WriteString("FULLTEXT INDEX ")
		s.WriteString(QuoteName(k.key))
	case keySpatial:
		s.WriteString("SPATIAL INDEX ")
		s.WriteString(QuoteName(k.key))
	default:
		return "" // ??
	}

	s.WriteByte('(')
	for n, f := range k.fields {
		if n > 0 {
			s.WriteString(", ")
		}
		s.WriteString(QuoteName(f))
	}
	s.WriteByte(')')
	return s.String()
}

func (k *structKey) isUnique() bool {
	switch k.typ {
	case keyPrimary, keyUnique:
		return true
	default:
		return false
	}
}

func (k *structKey) matches(otherK *keyinfo) (bool, error) {
	// check if this key matches
	// For now, just return true as the original method did
	return true, nil
}

func (k *structKey) matchesPG(otherK *pgShowIndex) (bool, error) {
	// check if this key matches
	// ColumnName?
	return true, nil
}
