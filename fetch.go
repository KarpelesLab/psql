package psql

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"strings"
)

func FetchOne(ctx context.Context, target any, where map[string]any) error {
	table := GetTableMeta(reflect.TypeOf(target))
	return table.FetchOne(ctx, target, where)
}

func Fetch(ctx context.Context, obj any, where map[string]any) ([]any, error) {
	table := GetTableMeta(reflect.TypeOf(obj))
	return table.Fetch(ctx, where)
}

func (t *TableMeta) FetchOne(ctx context.Context, target interface{}, where map[string]interface{}) error {
	// grab fields from target
	val := reflect.ValueOf(target)

	if val.Kind() != reflect.Ptr {
		panic(fmt.Sprintf("target must be a pointer to a struct, got a %T", target))
	}
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
