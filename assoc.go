package psql

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"strings"
)

type assocKind int

const (
	assocBelongsTo assocKind = iota
	assocHasOne
	assocHasMany
)

type assocMeta struct {
	index      int // field index in parent struct
	kind       assocKind
	foreignKey string       // FK column name
	targetType reflect.Type // element type (e.g., User, not *User or []*User)
	fieldName  string       // Go struct field name
}

// assocFetcher is an internal interface implemented by TableMeta[T] for association preloading.
type assocFetcher interface {
	assocFetchByColumn(ctx context.Context, column string, keys []any) (map[any][]reflect.Value, error)
	assocPrimaryKeyCol() string
}

// Preload loads associations for the given targets.
// Association fields must be declared with psql struct tags (e.g., `psql:"belongs_to:UserID"`).
// Target types for associations must have been registered via Table[T]().
func Preload[T any](ctx context.Context, targets []*T, fields ...string) error {
	if len(targets) == 0 {
		return nil
	}
	t := Table[T]()
	if t == nil {
		return ErrNotReady
	}
	for _, fieldName := range fields {
		assoc, ok := t.assocs[fieldName]
		if !ok {
			return fmt.Errorf("unknown association %q on type %s", fieldName, t.typ.Name())
		}
		vals := make([]reflect.Value, len(targets))
		for i, target := range targets {
			vals[i] = reflect.ValueOf(target).Elem()
		}
		if err := assoc.preload(ctx, t.fldcol, t.mainKey, vals); err != nil {
			return err
		}
	}
	return nil
}

// WithPreload returns a FetchOptions that automatically preloads the given associations after fetching.
func WithPreload(fields ...string) *FetchOptions {
	return &FetchOptions{Preload: fields}
}

func parseAssocTag(tag string, finfo reflect.StructField, index int) *assocMeta {
	parts := strings.SplitN(tag, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		slog.Warn(fmt.Sprintf("[psql] invalid psql tag format, expected kind:ForeignKey"), "event", "psql:assoc:bad_tag", "field", finfo.Name, "tag", tag)
		return nil
	}

	var kind assocKind
	switch strings.ToLower(parts[0]) {
	case "belongs_to":
		kind = assocBelongsTo
	case "has_one":
		kind = assocHasOne
	case "has_many":
		kind = assocHasMany
	default:
		slog.Warn(fmt.Sprintf("[psql] unknown association type %q", parts[0]), "event", "psql:assoc:bad_kind", "field", finfo.Name)
		return nil
	}

	targetType := finfo.Type
	if targetType.Kind() == reflect.Slice {
		targetType = targetType.Elem()
	}
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}

	if kind == assocHasMany && finfo.Type.Kind() != reflect.Slice {
		slog.Warn("[psql] has_many association must be a slice type", "event", "psql:assoc:bad_slice", "field", finfo.Name)
		return nil
	}

	return &assocMeta{
		index:      index,
		kind:       kind,
		foreignKey: parts[1],
		targetType: targetType,
		fieldName:  finfo.Name,
	}
}

func (a *assocMeta) preload(ctx context.Context, parentFldcol map[string]*structField, parentKey *structKey, targets []reflect.Value) error {
	tableMapL.RLock()
	targetTable, ok := tableMap[a.targetType]
	tableMapL.RUnlock()
	if !ok {
		return fmt.Errorf("table for type %s not registered, ensure psql.Table[%s]() is called first", a.targetType.Name(), a.targetType.Name())
	}

	loader, ok := targetTable.(assocFetcher)
	if !ok {
		return fmt.Errorf("table for type %s does not support preloading", a.targetType.Name())
	}

	switch a.kind {
	case assocBelongsTo:
		return a.preloadBelongsTo(ctx, parentFldcol, targets, loader)
	case assocHasOne:
		return a.preloadHasOne(ctx, parentKey, parentFldcol, targets, loader)
	case assocHasMany:
		return a.preloadHasMany(ctx, parentKey, parentFldcol, targets, loader)
	}
	return nil
}

func (a *assocMeta) preloadBelongsTo(ctx context.Context, parentFldcol map[string]*structField, targets []reflect.Value, loader assocFetcher) error {
	fkField := findFieldByNameOrCol(parentFldcol, a.foreignKey)
	if fkField == nil {
		return fmt.Errorf("foreign key column %q not found", a.foreignKey)
	}

	keySet := make(map[any]struct{})
	for _, target := range targets {
		fkVal := target.Field(fkField.index)
		if fkVal.Kind() == reflect.Ptr && fkVal.IsNil() {
			continue
		}
		keySet[fkVal.Interface()] = struct{}{}
	}
	if len(keySet) == 0 {
		return nil
	}
	keys := make([]any, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}

	pkCol := loader.assocPrimaryKeyCol()
	if pkCol == "" {
		return fmt.Errorf("target type %s has no single-column primary key", a.targetType.Name())
	}

	resultMap, err := loader.assocFetchByColumn(ctx, pkCol, keys)
	if err != nil {
		return err
	}

	for _, target := range targets {
		fkVal := target.Field(fkField.index)
		if fkVal.Kind() == reflect.Ptr && fkVal.IsNil() {
			continue
		}
		fk := fkVal.Interface()
		if results, ok := resultMap[fk]; ok && len(results) > 0 {
			target.Field(a.index).Set(results[0])
		}
	}
	return nil
}

func (a *assocMeta) preloadHasOne(ctx context.Context, parentKey *structKey, parentFldcol map[string]*structField, targets []reflect.Value, loader assocFetcher) error {
	if parentKey == nil || len(parentKey.fields) != 1 {
		return fmt.Errorf("parent must have a single-column primary key for has_one")
	}
	pkCol := parentKey.fields[0]
	pkField := parentFldcol[pkCol]

	keySet := make(map[any]struct{})
	for _, target := range targets {
		keySet[target.Field(pkField.index).Interface()] = struct{}{}
	}
	keys := make([]any, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}

	resultMap, err := loader.assocFetchByColumn(ctx, a.foreignKey, keys)
	if err != nil {
		return err
	}

	for _, target := range targets {
		pk := target.Field(pkField.index).Interface()
		if results, ok := resultMap[pk]; ok && len(results) > 0 {
			target.Field(a.index).Set(results[0])
		}
	}
	return nil
}

func (a *assocMeta) preloadHasMany(ctx context.Context, parentKey *structKey, parentFldcol map[string]*structField, targets []reflect.Value, loader assocFetcher) error {
	if parentKey == nil || len(parentKey.fields) != 1 {
		return fmt.Errorf("parent must have a single-column primary key for has_many")
	}
	pkCol := parentKey.fields[0]
	pkField := parentFldcol[pkCol]

	keySet := make(map[any]struct{})
	for _, target := range targets {
		keySet[target.Field(pkField.index).Interface()] = struct{}{}
	}
	keys := make([]any, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}

	resultMap, err := loader.assocFetchByColumn(ctx, a.foreignKey, keys)
	if err != nil {
		return err
	}

	for _, target := range targets {
		pk := target.Field(pkField.index).Interface()
		if results, ok := resultMap[pk]; ok {
			sliceType := target.Field(a.index).Type()
			slice := reflect.MakeSlice(sliceType, len(results), len(results))
			for i, r := range results {
				slice.Index(i).Set(r)
			}
			target.Field(a.index).Set(slice)
		}
	}
	return nil
}

func findFieldByNameOrCol(fldcol map[string]*structField, name string) *structField {
	if f, ok := fldcol[name]; ok {
		return f
	}
	for _, f := range fldcol {
		if f.name == name {
			return f
		}
	}
	return nil
}

// assocFetcher implementation on TableMeta

func (t *TableMeta[T]) assocFetchByColumn(ctx context.Context, column string, keys []any) (map[any][]reflect.Value, error) {
	results, err := t.Fetch(ctx, map[string]any{column: keys})
	if err != nil {
		return nil, err
	}
	fld, ok := t.fldcol[column]
	if !ok {
		return nil, fmt.Errorf("column %q not found in table %s", column, t.table)
	}
	m := make(map[any][]reflect.Value)
	for _, r := range results {
		val := reflect.ValueOf(r).Elem()
		key := val.Field(fld.index).Interface()
		m[key] = append(m[key], reflect.ValueOf(r))
	}
	return m, nil
}

func (t *TableMeta[T]) assocPrimaryKeyCol() string {
	if t.mainKey != nil && len(t.mainKey.fields) == 1 {
		return t.mainKey.fields[0]
	}
	return ""
}
