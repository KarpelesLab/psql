package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
	"sync"
)

type TableMeta struct {
	typ    reflect.Type
	table  string // table name
	fields []*structField
	fldStr string // string of all fields
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
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		panic(fmt.Sprintf("target must be a *struct, got a %s", typ))
	}

	tableMapL.RLock()
	info, ok := tableMap[typ]
	tableMapL.RUnlock()
	if ok {
		return info
	}

	info = &TableMeta{
		typ: typ,
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
			tagData := strings.Split(tag, ",")
			if tagData[0] != "" {
				// could be sql:",type=..." so only set col if not empty
				col = tagData[0]
			}
			tagData = tagData[1:]
			for _, v := range tagData {
				p := strings.IndexByte(v, '=')
				if p == -1 {
					attrs[v] = ""
					continue
				}
				attrs[v[:p]] = v[p+1:]
			}
		}

		if finfo.Type == nameType {
			// this is actually the name of the table
			info.table = col
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

func (t *TableMeta) FetchOne(ctx context.Context, target interface{}, where map[string]interface{}) error {
	// grab fields from target
	val := reflect.ValueOf(target)

	if val.Kind() != reflect.Ptr {
		panic("target must be a *struct")
	}

	val = val.Elem()
	typ := val.Type()

	if typ != t.typ {
		panic("invalid type for query")
	}

	// SELECT QUERY
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []interface{}
	if where != nil {
		var whQ []string
		for k, v := range where {
			whQ = append(whQ, QuoteName(k)+"=?")
			params = append(params, v)
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}
	req += " LIMIT 1"

	// run query
	rows, err := db.QueryContext(ctx, req, params...)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return &Error{Query: req, Err: err}
	}
	defer rows.Close()

	if !rows.Next() {
		// no result
		return os.ErrNotExist
	}

	err = t.scanValue(rows, val)
	return err
}

func (t *TableMeta) Fetch(ctx context.Context, where map[string]interface{}) ([]interface{}, error) {
	// SELECT QUERY
	req := "SELECT " + t.fldStr + " FROM " + QuoteName(t.table)
	var params []interface{}
	if where != nil {
		var whQ []string
		for k, v := range where {
			whQ = append(whQ, QuoteName(k)+"=?")
			params = append(params, v)
		}
		if len(whQ) > 0 {
			req += " WHERE " + strings.Join(whQ, " AND ")
		}
	}

	// run query
	rows, err := db.QueryContext(ctx, req, params...)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return nil, &Error{Query: req, Err: err}
	}
	defer rows.Close()

	var final []interface{}

	for rows.Next() {
		val := reflect.New(t.typ)

		err = t.scanValue(rows, val.Elem())
		if err != nil {
			return nil, err
		}
		final = append(final, val.Interface())
	}

	return final, nil
}

func (t *TableMeta) Insert(ctx context.Context, targets ...any) error {
	// INSERT QUERY
	req := "INSERT INTO " + QuoteName(t.table) + " (" + t.fldStr + ") VALUES (" + strings.TrimSuffix(strings.Repeat("?,", len(t.fields)), ",") + ")"
	stmt, err := db.PrepareContext(ctx, req)
	if err != nil {
		log.Printf("[sql] error: %s", err)
		return &Error{Query: req, Err: err}
	}
	defer stmt.Close()

	for _, target := range targets {
		val := reflect.ValueOf(target)

		if val.Kind() == reflect.Ptr {
			val = val.Elem()
		}
		typ := val.Type()

		if typ != t.typ {
			panic("invalid type for query")
		}

		params := make([]any, len(t.fields))

		for n, f := range t.fields {
			fval := val.Field(f.index)
			if fval.Kind() == reflect.Ptr {
				if fval.IsNil() {
					continue
				}
			}
			params[n] = export(fval.Interface())
		}

		_, err := stmt.ExecContext(ctx, params...)
		if err != nil {
			log.Printf("[sql] error: %s", err)
			return &Error{Query: req, Err: err}
		}
	}
	return nil
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
