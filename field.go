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

type structField struct {
	index    int
	name     string
	column   string // column name, can be != name
	nullable bool   // if a ptr or a kind of nullable value
	attrs    map[string]string
	setter   func(v reflect.Value, from sql.RawBytes) error
	rattrs   map[Engine]map[string]string // resolved attrs
}

// getAttrs returns the fields' attrs for a given Engine, which can be cached for performance
func (f *structField) getAttrs(e Engine) map[string]string {
	if r, ok := f.rattrs[e]; ok {
		return r
	}
	f.rattrs[e] = f.resolveAttrs(e, f.attrs)
	return f.rattrs[e]
}

func (f *structField) resolveAttrs(e Engine, attrs map[string]string) map[string]string {
	// check for import
	if imp, ok := attrs["import"]; ok {
		var res map[string]string
		// load it from magic
		if magic, ok := magicEngineTypes[e][f.column+"+"+imp]; ok {
			// found a magic type
			res = f.resolveAttrs(e, parseAttrs(magic)) // recursive allowed
		} else if magic, ok := magicTypes[f.column+"+"+imp]; ok {
			// found a magic type
			res = f.resolveAttrs(e, parseAttrs(magic)) // recursive allowed
		} else if magic, ok := magicEngineTypes[e][imp]; ok {
			// found a magic type
			res = f.resolveAttrs(e, parseAttrs(magic)) // recursive allowed
		} else if magic, ok = magicTypes[imp]; ok {
			// found a magic type
			res = f.resolveAttrs(e, parseAttrs(magic)) // recursive allowed
		} else {
			res = make(map[string]string)
			slog.Error(fmt.Sprintf("[psql] could not find import type %s for field %s engine %s", imp, f.column, e), "event", "psql:field:attr:missing_import", "psql.field", f.name)
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

func (f *structField) sqlType(e Engine) string {
	attrs := f.getAttrs(e)
	if attrs == nil {
		return ""
	}

	mytyp, ok := attrs["type"]
	if !ok {
		return ""
	}

	mytyp = strings.ToLower(mytyp)

	switch mytyp {
	case "enum", "set":
		if e == EnginePostgreSQL {
			switch mytyp {
			case "enum":
				// TODO FIXME stopgap
				return "varchar(128)"
			case "set":
				// we return set but it will actually be a jsonb
				return "varchar(128)"
				//return "set"
			}
		}
		// get "values"
		if myvals, ok := attrs["values"]; ok {
			// split with ,
			l := strings.Split(myvals, ",")
			// assuming nothing need to be escape for enum/set values (TODO FIXME)
			return mytyp + "('" + strings.Join(l, "','") + "')"
		} else {
			// give up
			return ""
		}
	default:
		if e == EnginePostgreSQL {
			// pgsql requires int types to have no length
			if x, ok := numericTypes[mytyp]; ok && x {
				return mytyp
			}
		}
		if mysize, ok := attrs["size"]; ok {
			return mytyp + "(" + mysize + ")"
		}
	}

	return mytyp
}

/*
	column_definition: {
	    data_type [NOT NULL | NULL] [DEFAULT {literal | (expr)} ]
	      [VISIBLE | INVISIBLE]
	      [AUTO_INCREMENT] [UNIQUE [KEY]] [[PRIMARY] KEY]
	      [COMMENT 'string']
	      [COLLATE collation_name]
	      [COLUMN_FORMAT {FIXED | DYNAMIC | DEFAULT}]
	      [ENGINE_ATTRIBUTE [=] 'string']
	      [SECONDARY_ENGINE_ATTRIBUTE [=] 'string']
	      [STORAGE {DISK | MEMORY}]
	      [reference_definition]
	      [check_constraint_definition]
	  | data_type
	      [COLLATE collation_name]
	      [GENERATED ALWAYS] AS (expr)
	      [VIRTUAL | STORED] [NOT NULL | NULL]
	      [VISIBLE | INVISIBLE]
	      [UNIQUE [KEY]] [[PRIMARY] KEY]
	      [COMMENT 'string']
	      [reference_definition]
	      [check_constraint_definition]
	}
*/
func (f *structField) defString(e Engine) string {
	attrs := f.getAttrs(e)
	mytyp := f.sqlType(e)
	if mytyp == "" {
		return ""
	}
	setType := false

	if e == EnginePostgreSQL && mytyp == "set" {
		mytyp = "jsonb"
		setType = true
	}

	mydef := QuoteName(f.column) + " " + mytyp

	// TODO unsigned

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
		switch e {
		case EnginePostgreSQL:
			// there are various things to take into account if engine is pgsql
			if setType {
				// need to encode default as json
				js, _ := json.Marshal([]string{def})
				def = string(js)
			}
			if def == "\\N" {
				mydef += " DEFAULT NULL"
			} else {
				mydef += " DEFAULT " + Escape(def)
			}
		default:
			if def == "\\N" {
				mydef += " DEFAULT NULL"
			} else {
				mydef += " DEFAULT " + Escape(def)
			}
		}
	}

	if mycol, ok := attrs["collation"]; ok {
		mydef += " COLLATE " + mycol
	}

	return mydef
}

func (f *structField) matches(e Engine, typ, null string, col, dflt *string) (bool, error) {
	attrs := f.getAttrs(e)
	if attrs == nil {
		return false, errors.New("no valid field defined")
	}

	myType := f.sqlType(e)
	if a, b := numericTypes[typ]; myType != typ && a && b {
		// typ we got from mysql is different, but that might not be an issue
		if typ == strings.ToLower(attrs["type"]) {
			// we're good
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
		// bad collation → alter
		return false, nil
	}

	return true, nil
}
