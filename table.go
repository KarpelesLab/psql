package psql

import (
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
)

type TableMeta struct {
	typ     reflect.Type
	table   string // table name
	fields  []*structField
	keys    []*structKey
	mainKey *structKey
	fldStr  string // string of all fields
}

var (
	tableMap  = make(map[reflect.Type]*TableMeta)
	tableMapL sync.RWMutex
)

func getAllTableMeta() []*TableMeta {
	tableMapL.Lock()
	defer tableMapL.Unlock()

	res := make([]*TableMeta, 0, len(tableMap))

	for _, v := range tableMap {
		res = append(res, v)
	}
	return res
}

func Table(obj any) *TableMeta {
	return GetTableMeta(reflect.TypeOf(obj))
}

func GetTableMeta(typ reflect.Type) *TableMeta {
	for typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		panic(fmt.Sprintf("target must be a struct, got a %s", typ))
	}

	tableMapL.RLock()
	info, ok := tableMap[typ]
	tableMapL.RUnlock()
	if ok {
		return info
	}

	info = &TableMeta{
		typ:   typ,
		table: FormatTableName(typ.Name()),
	}

	cnt := typ.NumField()
	var names []string

	for i := 0; i < cnt; i += 1 {
		finfo := typ.Field(i)
		col := finfo.Name
		attrs := make(map[string]string)

		tag := finfo.Tag.Get("sql")
		if tag != "" {
			if tag == "-" {
				// skip
				continue
			}
			// handle properties, etc
			tagCol, tagAttrs := parseTagData(tag)
			if tagCol != "" {
				// could be sql:",type=..." so only set col if not empty
				col = tagCol
			}
			attrs = tagAttrs
		}

		switch finfo.Type {
		case nameType:
			// this is actually the name of the table
			info.table = col
			continue
		case keyType:
			key := &structKey{
				index: i,
				name:  finfo.Name,
				key:   col,
			}
			key.loadAttrs(attrs)
			info.keys = append(info.keys, key)

			if (info.mainKey == nil && key.isUnique()) || key.typ == keyPrimary {
				info.mainKey = key
			}
			continue
		}

		if len(attrs) == 0 {
			// import based on type
			attrs["import"] = finfo.Type.String()
		}

		fld := &structField{
			index:  i,
			name:   finfo.Name,
			column: col,
			setter: findSetter(finfo.Type),
		}
		fld.loadAttrs(attrs)
		names = append(names, QuoteName(col))

		// TODO handle other kind of nullables, such as sql.NullString etc
		if finfo.Type.Kind() == reflect.Ptr {
			fld.nullable = true
		}

		info.fields = append(info.fields, fld)
	}

	if len(info.fields) == 0 {
		panic("no fields for table")
	}

	info.fldStr = strings.Join(names, ",")

	tableMapL.Lock()
	tableMap[typ] = info
	tableMapL.Unlock()

	if err := info.checkStructure(); err != nil {
		log.Printf("psql: failed to check table %s: %s", info.table, err)
	}

	return info
}

func (t *TableMeta) Name() string {
	return t.table
}

func (t *TableMeta) scanValue(rows *sql.Rows, val reflect.Value) error {
	// Make a slice for the values, and a reference interface slice
	values := make([]sql.RawBytes, len(t.fields))
	scan := make([]interface{}, len(t.fields))
	for i := range values {
		scan[i] = &values[i]
	}

	// scan
	err := rows.Scan(scan...)
	if err != nil {
		log.Printf("scan err %s", err)
		return err
	}

	// perform set
	for i := 0; i < len(t.fields); i += 1 {
		f := val.Field(t.fields[i].index)
		// if nil, set to nil
		if values[i] == nil {
			if f.Kind() == reflect.Ptr {
				if !f.IsNil() {
					f.Set(reflect.Zero(f.Type()))
				}
				continue
			} else {
				return fmt.Errorf("on field %s: %w", t.fields[i].name, ErrNotNillable)
			}
		}
		// make sure "f" is a settable value (not a ptr), allocate if needed
		for f.Kind() == reflect.Ptr {
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			}
			f = f.Elem()
		}
		err = t.fields[i].setter(f, values[i])
		if err != nil {
			return fmt.Errorf("on field %s: %w", t.fields[i].name, err)
		}
	}

	return nil
}
