package psql

import (
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"strings"
	"sync"
)

type TableMeta[T any] struct {
	typ     reflect.Type
	table   string // table name
	fields  []*structField
	fldcol  map[string]*structField
	keys    []*structKey
	mainKey *structKey
	fldStr  string // string of all fields
	state   int
	attrs   map[string]string
}

type TableMetaIntf interface {
	Name() string
}

var (
	tableMap  = make(map[reflect.Type]TableMetaIntf)
	tableMapL sync.RWMutex
)

func Table[T any]() *TableMeta[T] {
	if db == nil {
		return nil
	}

	typ := reflect.TypeOf((*T)(nil)).Elem()

	if typ.Kind() != reflect.Struct {
		panic(fmt.Sprintf("target must be a *struct, got a %s", typ))
	}

	tableMapL.RLock()
	found, ok := tableMap[typ]
	tableMapL.RUnlock()
	if ok {
		return found.(*TableMeta[T])
	}

	info := &TableMeta[T]{
		typ:    typ,
		table:  FormatTableName(typ.Name()),
		fldcol: make(map[string]*structField),
		attrs:  make(map[string]string),
		state:  -1,
	}

	cnt := typ.NumField()
	var names []string
	extraKeys := make(map[string]*structKey)

	for i := 0; i < cnt; i += 1 {
		if !typ.Field(i).IsExported() {
			continue
		}
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
			if info.state == -1 {
				info.state = i
			}
			continue
		case keyType:
			if info.state == -1 {
				info.state = i
			}
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

		if keyName, ok := attrs["key"]; ok {
			delete(attrs, "key")
			if k, found := extraKeys[keyName]; found {
				k.fields = append(k.fields, col)
			} else {
				k = &structKey{
					index:  -1,
					fields: []string{col},
					attrs:  map[string]string{},
				}
				k.loadKeyName(keyName)
				extraKeys[keyName] = k
				info.keys = append(info.keys, k)

				if (info.mainKey == nil && k.isUnique()) || k.typ == keyPrimary {
					info.mainKey = k
				}
			}
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

func (t *TableMeta[T]) Name() string {
	if t == nil {
		return ""
	}
	return t.table
}

func (t *TableMeta[T]) newobj() *T {
	return reflect.New(t.typ).Interface().(*T)
}

func (t *TableMeta[T]) spawn(rows *sql.Rows) (*T, error) {
	// spawn an object based on the provided row
	res := t.newobj()
	err := t.scanValue(rows, res)
	return res, err
}

func (t *TableMeta[T]) ScanTo(row *sql.Rows, v *T) error {
	return t.scanValue(row, v)
}

func (t *TableMeta[T]) scanValue(rows *sql.Rows, target *T) error {
	val := reflect.ValueOf(target).Elem()
	st := t.rowstate(target)

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

	if st != nil {
		st.init = true
		st.val = make(map[string]any)
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
		if st != nil {
			v := reflect.New(f.Type()).Elem()
			vp := v
			for vp.Kind() == reflect.Ptr {
				if vp.IsNil() {
					vp.Set(reflect.New(vp.Type().Elem()))
				}
				vp = vp.Elem()
			}
			fld.setter(vp, values[i])
			st.val[cols[i]] = dupv(v.Interface())
		}
	}

	return nil
}
