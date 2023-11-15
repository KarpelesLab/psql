package psql

import (
	"database/sql"
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
}

func (f *structField) parseAttrs(tag string) {
	attrs := make(map[string]string)

	tagData := strings.Split(tag, ",")
	for _, v := range tagData {
		p := strings.IndexByte(v, '=')
		if p == -1 {
			attrs[v] = ""
			continue
		}
		attrs[v[:p]] = v[p+1:]
	}

	f.loadAttrs(attrs)
}

func (f *structField) loadAttrs(attrs map[string]string) {
	// check for import
	if imp, ok := attrs["import"]; ok {
		// load it from magic
		if magic, ok := magicTypes[f.column+"+"+imp]; ok {
			// found a magic type
			f.parseAttrs(magic) // recursive allowed
		} else if magic, ok = magicTypes[imp]; ok {
			// found a magic type
			f.parseAttrs(magic) // recursive allowed
		} else {
			slog.Error(fmt.Sprintf("[psql] could not find import type %s for field %s", imp, f.column), "event", "psql:field:attr:missing_import", "psql.field", f.name)
		}
	} else if _, ok = attrs["type"]; ok {
		// has a type, so can be used as is
		f.attrs = attrs
		return
	}

	if f.attrs != nil {
		// we managed to load something, apply remaining attrs
		for k, v := range attrs {
			f.attrs[k] = v
		}
	}
}

func (f *structField) sqlType() string {
	if f.attrs == nil {
		return ""
	}

	mytyp, ok := f.attrs["type"]
	if !ok {
		return ""
	}

	mytyp = strings.ToLower(mytyp)

	switch mytyp {
	case "enum", "set":
		// get "values"
		if myvals, ok := f.attrs["values"]; ok {
			// split with ,
			l := strings.Split(myvals, ",")
			// assuming nothing need to be escape for enum/set values (TODO FIXME)
			return mytyp + "('" + strings.Join(l, "','") + "')"
		} else {
			// give up
			return ""
		}
	default:
		if mysize, ok := f.attrs["size"]; ok {
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
func (f *structField) defString() string {
	mytyp := f.sqlType()
	if mytyp == "" {
		return ""
	}

	mydef := QuoteName(f.column) + " " + mytyp

	// TODO unsigned

	if null, ok := f.attrs["null"]; ok {
		switch null {
		case "0", "false":
			mydef += " NOT NULL"
		case "1", "true":
			mydef += " NULL"
		default:
			return "" // bad def
		}
	}
	if def, ok := f.attrs["default"]; ok {
		if def == "\\N" {
			mydef += " DEFAULT NULL"
		} else {
			mydef += " DEFAULT " + Escape(def)
		}
	}

	if mycol, ok := f.attrs["collation"]; ok {
		mydef += " COLLATE " + mycol
	}

	return mydef
}

func (f *structField) matches(typ, null string, col, dflt *string) (bool, error) {
	if f.attrs == nil {
		return false, errors.New("no valid field defined")
	}

	myType := f.sqlType()
	if a, b := numericTypes[typ]; myType != typ && a && b {
		// typ we got from mysql is different, but that might not be an issue
		if typ == strings.ToLower(f.attrs["type"]) {
			// we're good
			myType = typ
		}
	}

	if myType != typ {
		return false, nil
	}

	// check null
	if mynull, ok := f.attrs["null"]; ok {
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
	if mydef, ok := f.attrs["default"]; ok && dflt != nil && mydef != *dflt {
		return false, nil
	}

	if mycol, ok := f.attrs["collation"]; ok && col != nil && mycol != *col {
		// bad collation → alter
		return false, nil
	}

	return true, nil
}
