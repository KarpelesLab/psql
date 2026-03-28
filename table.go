package psql

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
	"sync"

	"github.com/KarpelesLab/typutil"
)

var (
	tableMap  = make(map[reflect.Type]TableMetaIntf)
	tableMapL sync.RWMutex
)

// TableView is a non-generic interface for accessing table metadata from
// dialect implementations (which cannot use type parameters).
type TableView interface {
	TableName() string
	FormattedName(be *Backend) string
	AllFields() []*StructField
	AllKeys() []*StructKey
	MainKey() *StructKey
	FieldByColumn(col string) *StructField
	FieldStr() string
	TableAttrs() map[string]string
	HasSoftDelete() bool
}

// TableMeta holds the metadata for a registered table type T, including its fields,
// keys, associations, and SQL column mappings. Obtain one via [Table].
type TableMeta[T any] struct {
	typ          reflect.Type
	table        string // table name
	explicitName bool   // true if table name was explicitly set via psql.Name
	fields       []*StructField
	fldcol       map[string]*StructField
	keys         []*StructKey
	mainKey      *StructKey
	fldStr       string // string of all fields
	state        int
	attrs        map[string]string
	futures      sync.Map
	assocs       map[string]*assocMeta // association metadata by Go field name
	softDelete   *StructField          // non-nil if soft delete is enabled
}

type TableMetaIntf interface {
	Name() string
}

// Verify TableMeta implements TableView.
var _ TableView = (*TableMeta[struct{}])(nil)

// Table returns the table object for T against DefaultBackend unless the provided
// ctx value has a backend.
func Table[T any]() *TableMeta[T] {
	typ := reflect.TypeFor[T]()

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
		fldcol: make(map[string]*StructField),
		attrs:  make(map[string]string),
		assocs: make(map[string]*assocMeta),
		state:  -1,
	}

	cnt := typ.NumField()
	var names []string
	extraKeys := make(map[string]*StructKey)

	for i := 0; i < cnt; i += 1 {
		if !typ.Field(i).IsExported() {
			continue
		}
		finfo := typ.Field(i)

		// Check for psql association tag
		if psqlTag := finfo.Tag.Get("psql"); psqlTag != "" {
			assoc := parseAssocTag(psqlTag, finfo, i)
			if assoc != nil {
				info.assocs[finfo.Name] = assoc
			}
			continue
		}

		col := finfo.Name
		attrs := make(map[string]string)

		// Column name transformations will happen at query time

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
				// Explicit tag name overrides the namer
				col = tagCol
			}
			attrs = tagAttrs
		}

		switch finfo.Type {
		case nameType:
			// this is actually the name of the table
			info.table = col
			info.explicitName = true // Mark that name was explicitly provided
			info.attrs = attrs
			if info.state == -1 {
				info.state = i
			}
			continue
		case keyType:
			if info.state == -1 {
				info.state = i
			}
			key := &StructKey{
				Index: i,
				Name:  finfo.Name,
				Key:   col,
			}
			key.loadAttrs(attrs)
			info.keys = append(info.keys, key)

			if (info.mainKey == nil && key.IsUnique()) || key.Typ == KeyPrimary {
				info.mainKey = key
			}
			continue
		}

		if keyName, ok := attrs["key"]; ok {
			delete(attrs, "key")
			if k, found := extraKeys[keyName]; found {
				k.Fields = append(k.Fields, col)
			} else {
				k = &StructKey{
					Index:  -1,
					Fields: []string{col},
					Attrs:  map[string]string{},
				}
				k.loadKeyName(keyName)
				extraKeys[keyName] = k
				info.keys = append(info.keys, k)

				if (info.mainKey == nil && k.IsUnique()) || k.Typ == KeyPrimary {
					info.mainKey = k
				}
			}
		}

		if len(attrs) == 0 {
			// import based on type
			attrs["import"] = finfo.Type.String()
		}

		var setter func(reflect.Value, sql.RawBytes) error
		if attrs["format"] == "json" {
			t := finfo.Type
			for t.Kind() == reflect.Ptr {
				t = t.Elem()
			}
			setter = makeJSONSetter(t)
		} else {
			setter = findSetter(finfo.Type)
		}

		fld := &StructField{
			Index:  i,
			Name:   finfo.Name,
			Column: col,
			setter: setter,
			Attrs:  attrs,
			Rattrs: make(map[Engine]map[string]string),
		}
		names = append(names, QuoteName(col))

		// TODO handle other kind of nullables, such as sql.NullString etc
		if finfo.Type.Kind() == reflect.Ptr {
			fld.Nullable = true
		}

		info.fields = append(info.fields, fld)
		info.fldcol[fld.Column] = fld

		// Detect soft delete: *time.Time field named "DeletedAt" or with softdelete attr
		if _, ok := attrs["softdelete"]; ok || (finfo.Name == "DeletedAt" && finfo.Type == ptrTimeType) {
			info.softDelete = fld
		}
	}

	if len(info.fields) == 0 {
		panic("no fields for table")
	}

	info.fldStr = strings.Join(names, ",")

	tableMapL.Lock()
	tableMap[typ] = info
	tableMapL.Unlock()

	return info
}

func (t *TableMeta[T]) Name() string {
	if t == nil {
		return ""
	}
	return t.table
}

// TableName returns the raw table name (implements TableView).
func (t *TableMeta[T]) TableName() string {
	return t.table
}

// FormattedName returns the table name, applying the namer transformation if needed
func (t *TableMeta[T]) FormattedName(be *Backend) string {
	if t == nil {
		return ""
	}
	if t.explicitName {
		// Table name was explicitly set via psql.Name, use as-is
		return t.table
	}
	// Apply namer transformation
	return be.Namer().TableName(t.table)
}

// AllFields returns all fields (implements TableView).
func (t *TableMeta[T]) AllFields() []*StructField {
	return t.fields
}

// AllKeys returns all keys (implements TableView).
func (t *TableMeta[T]) AllKeys() []*StructKey {
	return t.keys
}

// MainKey returns the primary/unique key (implements TableView).
func (t *TableMeta[T]) MainKey() *StructKey {
	return t.mainKey
}

// FieldByColumn returns a field by its column name (implements TableView).
func (t *TableMeta[T]) FieldByColumn(col string) *StructField {
	return t.fldcol[col]
}

// FieldStr returns the comma-separated quoted field list (implements TableView).
func (t *TableMeta[T]) FieldStr() string {
	return t.fldStr
}

// TableAttrs returns the table-level attributes (implements TableView).
func (t *TableMeta[T]) TableAttrs() map[string]string {
	return t.attrs
}

// HasSoftDelete returns true if the table has a soft delete field (implements TableView).
func (t *TableMeta[T]) HasSoftDelete() bool {
	return t.softDelete != nil
}

func (t *TableMeta[T]) newobj() *T {
	return reflect.New(t.typ).Interface().(*T)
}

func (t *TableMeta[T]) spawnAll(ctx context.Context, rows *sql.Rows) ([]*T, error) {
	var res []*T
	defer rows.Close()

	for rows.Next() {
		obj := t.newobj()
		err := t.scanValue(ctx, rows, obj)
		if err != nil {
			return res, err
		}
		res = append(res, obj)
	}
	return res, nil
}

func (t *TableMeta[T]) spawn(ctx context.Context, rows *sql.Rows) (*T, error) {
	// spawn an object based on the provided row
	res := t.newobj()
	err := t.scanValue(ctx, rows, res)
	return res, err
}

func (t *TableMeta[T]) ScanTo(ctx context.Context, row *sql.Rows, v *T) error {
	return t.scanValue(ctx, row, v)
}

func (t *TableMeta[T]) scanValue(ctx context.Context, rows *sql.Rows, target *T) error {
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
		slog.Error(fmt.Sprintf("scan err %s", err), "event", "psql:table:scan_error", "psql.table", t.table)
		return fmt.Errorf("scan error: %w", err)
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
		f := val.Field(fld.Index)
		// if nil, set to nil
		if values[i] == nil {
			if f.Kind() == reflect.Ptr {
				if !f.IsNil() {
					f.Set(reflect.Zero(f.Type()))
				}
			}
			continue
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
			return fmt.Errorf("on field %s: %w", fld.Name, err)
		}
		if st != nil {
			if fld.Attrs["format"] == "json" {
				// Store raw JSON for state; DeepClone can't handle map[string]any
				st.val[cols[i]] = string(values[i])
			} else {
				v := reflect.New(f.Type()).Elem()
				vp := v
				for vp.Kind() == reflect.Ptr {
					if vp.IsNil() {
						vp.Set(reflect.New(vp.Type().Elem()))
					}
					vp = vp.Elem()
				}
				fld.setter(vp, values[i])
				st.val[cols[i]] = typutil.DeepClone(v.Interface())
			}
		}
	}

	if h, ok := any(target).(AfterScanHook); ok {
		if err := h.AfterScan(ctx); err != nil {
			return err
		}
	}

	return nil
}

// scanValueReturning is like scanValue but skips AfterScanHook. Used for
// RETURNING clauses where the scan is part of an INSERT/REPLACE, not a SELECT.
func (t *TableMeta[T]) scanValueReturning(ctx context.Context, rows *sql.Rows, target *T) error {
	val := reflect.ValueOf(target).Elem()
	st := t.rowstate(target)

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

	err = rows.Scan(scan...)
	if err != nil {
		slog.Error(fmt.Sprintf("scan err %s", err), "event", "psql:table:scan_error", "psql.table", t.table)
		return fmt.Errorf("scan error: %w", err)
	}

	if st != nil {
		st.init = true
		st.val = make(map[string]any)
	}

	for i := 0; i < n; i += 1 {
		fld, ok := t.fldcol[cols[i]]
		if !ok {
			continue
		}
		f := val.Field(fld.Index)
		if values[i] == nil {
			if f.Kind() == reflect.Ptr {
				if !f.IsNil() {
					f.Set(reflect.Zero(f.Type()))
				}
			}
			continue
		}
		for f.Kind() == reflect.Ptr {
			if f.IsNil() {
				f.Set(reflect.New(f.Type().Elem()))
			}
			f = f.Elem()
		}
		err = fld.setter(f, values[i])
		if err != nil {
			return fmt.Errorf("on field %s: %w", fld.Name, err)
		}
		if st != nil {
			if fld.Attrs["format"] == "json" {
				st.val[cols[i]] = string(values[i])
			} else {
				v := reflect.New(f.Type()).Elem()
				vp := v
				for vp.Kind() == reflect.Ptr {
					if vp.IsNil() {
						vp.Set(reflect.New(vp.Type().Elem()))
					}
					vp = vp.Elem()
				}
				fld.setter(vp, values[i])
				st.val[cols[i]] = typutil.DeepClone(v.Interface())
			}
		}
	}

	// AfterScanHook is intentionally NOT called here since this is a
	// RETURNING scan, not a user-initiated SELECT.
	return nil
}
