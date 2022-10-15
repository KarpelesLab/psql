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
	fldcol  map[string]*structField
	keys    []*structKey
	mainKey *structKey
	fldStr  string // string of all fields
	attrs   map[string]string
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
		typ:    typ,
		table:  FormatTableName(typ.Name()),
		fldcol: make(map[string]*structField),
		attrs:  make(map[string]string),
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
			info.attrs = attrs
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
		info.fldcol[fld.column] = fld
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

func (t *TableMeta) spawn(rows *sql.Rows) (any, error) {
	// spawn an object based on the provided row
	val := reflect.New(t.typ)
	err := t.scanValue(rows, val)
	if err != nil {
		return nil, err
	}
	p := reflect.New(reflect.PointerTo(t.typ))
	p.Set(val)
	return p.Interface(), nil
}

func (t *TableMeta) ScanTo(row *sql.Rows, v any) error {
	val := reflect.ValueOf(v)
	for val.Kind() == reflect.Ptr {
		if val.IsNil() {
			// instanciate it
			val.Set(reflect.New(val.Type().Elem()))
		}
		val = val.Elem()
	}

	typ := val.Type()

	if typ != t.typ {
		panic("invalid type for query")
	}

	return t.scanValue(row, val)
}

func (t *TableMeta) scanValue(rows *sql.Rows, val reflect.Value) error {
	// Make a slice for the values, and a reference interface slice
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	n := len(cols)

	values := make([]sql.RawBytes, n)
	scan := make([]interface{}, n)
	for i := range values {
		scan[i] = &values[i]
	}

	// scan
	err = rows.Scan(scan...)
	if err != nil {
		log.Printf("scan err %s", err)
		return err
	}

	// perform set
	for i := 0; i < n; i += 1 {
		fld, ok := t.fldcol[cols[i]]
		if !ok {
			// maybe report this as a warning?
			continue
		}
		f := val.Field(fld.index)
		// if nil, set to nil
		if values[i] == nil {
			if f.Kind() == reflect.Ptr {
				if !f.IsNil() {
					f.Set(reflect.Zero(f.Type()))
				}
				continue
			} else {
				return fmt.Errorf("on field %s: %w", fld.name, ErrNotNillable)
			}
		}
		// make sure "f" is a settable value (not a ptr), allocate if needed
		for f.Kind() == reflect.Ptr {
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			}
			f = f.Elem()
		}
		err = fld.setter(f, values[i])
		if err != nil {
			return fmt.Errorf("on field %s: %w", fld.name, err)
		}
	}

	return nil
}
