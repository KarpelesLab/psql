package psql

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

// StructField holds metadata for a single table field/column, including its
// Go struct index, SQL column name, type attributes, and scan function.
type StructField struct {
	Index    int
	Name     string
	Column   string // column name, can be != name
	Nullable bool   // if a ptr or a kind of nullable value
	Attrs    map[string]string
	setter   func(v reflect.Value, from sql.RawBytes) error
	Rattrs   map[Engine]map[string]string // resolved attrs
}

// GetAttrs returns the fields' attrs for a given Engine, which can be cached for performance
func (f *StructField) GetAttrs(be *Backend) map[string]string {
	if r, ok := f.Rattrs[be.Engine()]; ok {
		return r
	}
	f.Rattrs[be.Engine()] = f.resolveAttrs(be, f.Attrs)
	return f.Rattrs[be.Engine()]
}

func (f *StructField) resolveAttrs(be *Backend, attrs map[string]string) map[string]string {
	// check for import
	if imp, ok := attrs["import"]; ok {
		var res map[string]string
		// load it from magic
		if magic, ok := magicEngineTypes[be.Engine()][f.Column+"+"+imp]; ok {
			// found a magic type
			res = f.resolveAttrs(be, parseAttrs(magic)) // recursive allowed
		} else if magic, ok := magicTypes[f.Column+"+"+imp]; ok {
			// found a magic type
			res = f.resolveAttrs(be, parseAttrs(magic)) // recursive allowed
		} else if magic, ok := magicEngineTypes[be.Engine()][imp]; ok {
			// found a magic type
			res = f.resolveAttrs(be, parseAttrs(magic)) // recursive allowed
		} else if magic, ok = magicTypes[imp]; ok {
			// found a magic type
			res = f.resolveAttrs(be, parseAttrs(magic)) // recursive allowed
		} else {
			res = make(map[string]string)
			slog.Error(fmt.Sprintf("[psql] could not find import type %s for field %s engine %s", imp, f.Column, be.Engine()), "event", "psql:field:attr:missing_import", "psql.field", f.Name)
		}

		// override any values from the import
		for k, v := range attrs {
			res[k] = v
		}
		return res
	} else if _, ok = attrs["type"]; ok {
		// has a type, so can be used as is
		return attrs
	}

	// couldn't resolve this
	// TODO raise error
	return attrs
}

// SqlType returns the SQL type string for this field, dispatching to the
// engine's TypeMapper if available.
func (f *StructField) SqlType(be *Backend) string {
	attrs := f.GetAttrs(be)
	if attrs == nil {
		return ""
	}

	mytyp, ok := attrs["type"]
	if !ok {
		return ""
	}

	mytyp = strings.ToLower(mytyp)

	// Check if dialect implements TypeMapper
	d := be.Engine().dialect()
	if tm, ok := d.(TypeMapper); ok {
		return tm.SqlType(mytyp, attrs)
	}

	// Generic default: type(size)
	if mysize, ok := attrs["size"]; ok {
		return mytyp + "(" + mysize + ")"
	}

	return mytyp
}

// DefString returns the full column definition SQL for this field, dispatching
// to the engine's TypeMapper if available.
func (f *StructField) DefString(be *Backend) string {
	attrs := f.GetAttrs(be)
	mytyp := f.SqlType(be)
	if mytyp == "" {
		return ""
	}

	// Check if dialect implements TypeMapper
	d := be.Engine().dialect()
	if tm, ok := d.(TypeMapper); ok {
		return tm.FieldDef(f.Column, mytyp, f.Nullable, attrs)
	}

	// Generic default
	return f.genericDefString(be, mytyp, attrs)
}

// DefStringAlter returns a column definition suitable for ALTER TABLE ADD COLUMN.
// If the dialect implements TypeMapper, it delegates to FieldDefAlter; otherwise
// falls back to DefString.
func (f *StructField) DefStringAlter(be *Backend) string {
	attrs := f.GetAttrs(be)
	mytyp := f.SqlType(be)
	if mytyp == "" {
		return ""
	}

	d := be.Engine().dialect()
	if tm, ok := d.(TypeMapper); ok {
		return tm.FieldDefAlter(f.Column, mytyp, f.Nullable, attrs)
	}

	return f.genericDefString(be, mytyp, attrs)
}

// genericDefString builds a generic column definition (MySQL-like).
func (f *StructField) genericDefString(be *Backend, mytyp string, attrs map[string]string) string {
	setType := false

	if be.Engine() == EnginePostgreSQL && mytyp == "set" {
		mytyp = "jsonb"
		setType = true
	}

	mydef := QuoteName(f.Column) + " " + mytyp

	if null, ok := attrs["null"]; ok {
		switch null {
		case "0", "false":
			mydef += " NOT NULL"
		case "1", "true":
			mydef += " NULL"
		default:
			return "" // bad def
		}
	}
	if def, ok := attrs["default"]; ok {
		if be.Engine() == EnginePostgreSQL && setType {
			js, _ := json.Marshal([]string{def})
			def = string(js)
		}
		if def == "\\N" {
			mydef += " DEFAULT NULL"
		} else {
			mydef += " DEFAULT " + Escape(def)
		}
	}

	if mycol, ok := attrs["collation"]; ok {
		if be.Engine() != EngineSQLite {
			mydef += " COLLATE " + mycol
		}
	}

	return mydef
}

// Matches checks if this field matches the given database column properties.
func (f *StructField) Matches(be *Backend, typ, null string, col, dflt *string) (bool, error) {
	attrs := f.GetAttrs(be)
	if attrs == nil {
		return false, errors.New("no valid field defined")
	}

	myType := f.SqlType(be)
	if a, b := NumericTypes[typ]; myType != typ && a && b {
		if typ == strings.ToLower(attrs["type"]) {
			myType = typ
		}
	}

	if myType != typ {
		return false, nil
	}

	// check null
	if mynull, ok := attrs["null"]; ok {
		switch mynull {
		case "0", "false":
			if null != "" && null != "NO" {
				return false, nil
			}
		case "1", "true":
			if null != "YES" {
				return false, nil
			}
		}
	}
	// check default
	if mydef, ok := attrs["default"]; ok && dflt != nil && mydef != *dflt {
		return false, nil
	}

	if mycol, ok := attrs["collation"]; ok && col != nil && mycol != *col {
		return false, nil
	}

	return true, nil
}
