package psql

import (
	"log"
	"reflect"
	"strings"
)

var keyType = reflect.TypeOf(Key{})

// Name allows specifying the table name when associating a table with a struct
//
// For example:
// type X struct {
// KeyName psql.Key `sql:",type=UNIQUE,fields='A,B'"`
// ...
// }
type Key struct{}

const (
	keyPrimary = iota + 1
	keyUnique
	keyIndex
	keyFulltext
	keySpatial
)

type structKey struct {
	index int
	name  string
	key   string // key name, can be != name
	typ   int
	attrs map[string]string
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
		default:
			log.Printf("[psql] Unsupported index key type %s assumed as INDEX", t)
		}
	} else if k.key == "PRIMARY" {
		k.typ = keyPrimary
	}
	k.attrs = attrs
}

func (k *structKey) keyname() string {
	if k.typ == keyPrimary {
		return "PRIMARY"
	}
	return k.key
}

func (k *structKey) defString() string {
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
	flds := k.attrs["fields"]
	for n, f := range strings.Split(flds, ",") {
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
