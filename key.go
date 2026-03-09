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

// Key type constants for StructKey.Typ.
const (
	KeyPrimary  = 1
	KeyUnique   = 2
	KeyIndex    = 3
	KeyFulltext = 4
	KeySpatial  = 5
	KeyVector   = 6
)

// StructKey holds metadata for a table key/index, including its type, column
// list, and attributes.
type StructKey struct {
	Index  int
	Name   string
	Key    string // key name, can be != name
	Typ    int
	Attrs  map[string]string
	Fields []string
}

func (k *StructKey) loadAttrs(attrs map[string]string) {
	k.Typ = KeyIndex // default value
	if t, ok := attrs["type"]; ok {
		switch strings.ToUpper(t) {
		case "PRIMARY":
			k.Typ = KeyPrimary
		case "UNIQUE":
			k.Typ = KeyUnique
		case "INDEX":
			k.Typ = KeyIndex
		case "FULLTEXT":
			k.Typ = KeyFulltext
		case "SPATIAL":
			k.Typ = KeySpatial
		case "VECTOR":
			k.Typ = KeyVector
		default:
			slog.Warn(fmt.Sprintf("[psql] Unsupported index key type %s assumed as INDEX", t), "event", "psql:key:badkey", "psql.index", k.Name)
		}
	} else if k.Key == "PRIMARY" {
		k.Typ = KeyPrimary
	}
	k.Attrs = attrs
	k.Fields = strings.Split(attrs["fields"], ",")
}

func (k *StructKey) loadKeyName(kn string) {
	switch {
	case kn == "PRIMARY":
		k.Typ = KeyPrimary
	case strings.HasPrefix(kn, "UNIQUE:"):
		kn = strings.TrimPrefix(kn, "UNIQUE:")
		k.Typ = KeyUnique
	}
	k.Name = kn
	k.Key = kn
}

// Keyname returns "PRIMARY" for primary keys, or the key name otherwise.
func (k *StructKey) Keyname() string {
	if k.Typ == KeyPrimary {
		return "PRIMARY"
	}
	return k.Key
}

// SqlKeyName returns "PRIMARY KEY" for primary keys, or "INDEX keyname" otherwise.
func (k *StructKey) SqlKeyName() string {
	if k.Typ == KeyPrimary {
		return "PRIMARY KEY"
	}
	return "INDEX " + QuoteName(k.Key)
}

// DefString returns the key definition SQL, dispatching to the engine's
// KeyRenderer if available.
func (k *StructKey) DefString(be *Backend) string {
	d := be.Engine().dialect()
	if kr, ok := d.(KeyRenderer); ok {
		return kr.KeyDef(k, "")
	}
	// Generic default: MySQL-like inline definition
	return k.genericDefString()
}

// InlineDefString returns the inline constraint definition for CREATE TABLE.
func (k *StructKey) InlineDefString(be *Backend, tableName string) string {
	d := be.Engine().dialect()
	if kr, ok := d.(KeyRenderer); ok {
		return kr.InlineKeyDef(k, tableName)
	}
	return k.genericDefString()
}

// CreateIndexSQL returns a CREATE INDEX statement, or empty string if the key
// should be created inline.
func (k *StructKey) CreateIndexSQL(be *Backend, tableName string) string {
	d := be.Engine().dialect()
	if kr, ok := d.(KeyRenderer); ok {
		return kr.CreateIndex(k, tableName)
	}
	return "" // generic default: all keys are inline
}

// genericDefString generates a MySQL-like inline key definition.
func (k *StructKey) genericDefString() string {
	s := &strings.Builder{}

	switch k.Typ {
	case KeyPrimary:
		s.WriteString("PRIMARY KEY ")
	case KeyUnique:
		s.WriteString("UNIQUE INDEX ")
		s.WriteString(QuoteName(k.Key))
	case KeyIndex:
		s.WriteString("INDEX ")
		s.WriteString(QuoteName(k.Key))
	case KeyFulltext:
		s.WriteString("FULLTEXT INDEX ")
		s.WriteString(QuoteName(k.Key))
	case KeySpatial:
		s.WriteString("SPATIAL INDEX ")
		s.WriteString(QuoteName(k.Key))
	case KeyVector:
		return ""
	default:
		return ""
	}

	s.WriteByte('(')
	for n, f := range k.Fields {
		if n > 0 {
			s.WriteString(", ")
		}
		s.WriteString(QuoteName(f))
	}
	s.WriteByte(')')
	return s.String()
}

// IsUnique returns true if this key is a primary key or unique index.
func (k *StructKey) IsUnique() bool {
	switch k.Typ {
	case KeyPrimary, KeyUnique:
		return true
	default:
		return false
	}
}
