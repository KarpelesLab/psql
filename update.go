package psql

import (
	"context"
	"errors"
	"reflect"
)

// Update is a short way to insert objects into database
//
// psql.Update(ctx, obj)
//
// Is equivalent to:
//
// psql.Table(obj).Update(ctx, obj)
//
// All passed objects must be of the same type
func Update[T any](ctx context.Context, target ...*T) error {
	if len(target) == 0 {
		return nil
	}

	return Table[T]().Update(ctx, target...)
}

func (t *TableMeta[T]) Update(ctx context.Context, target ...*T) error {
	if t == nil {
		return ErrNotReady
	}
	t.check(ctx)
	if t.mainKey == nil {
		return errors.New("cannot update values without a unique key")
	}

	for _, obj := range target {
		// check for changed values
		upd := make(map[string]any)

		val := reflect.ValueOf(obj).Elem()

		st := t.rowstate(obj)
		if st == nil || !st.init {
			// we don't have a state → update everything
			for _, f := range t.fields {
				upd[f.column] = val.Field(f.index).Interface()
			}
		} else {
			for _, f := range t.fields {
				// grab state value
				stv, ok := st.val[f.column]
				newv := val.Field(f.index).Interface()

				if !ok {
					// no value in state → just force update
					upd[f.column] = newv
					continue
				}
				if !reflect.DeepEqual(newv, stv) {
					upd[f.column] = newv
				}
			}
		}
		if len(upd) == 0 {
			// no update needed
			continue
		}
		// perform update
		req := "UPDATE " + QuoteName(t.table) + " SET "
		var flds []any
		first := true
		for k, v := range upd {
			if !first {
				req += ", "
			} else {
				first = false
			}
			req += QuoteName(k) + " = ?"
			flds = append(flds, export(v))
		}
		req += " WHERE "
		first = true
		// render key
		for _, col := range t.mainKey.fields {
			if !first {
				req += " AND "
			} else {
				first = false
			}
			req += QuoteName(col) + " = ?"
			flds = append(flds, export(val.Field(t.fldcol[col].index).Interface()))
		}

		_, err := ExecContext(ctx, req, flds...)
		if err != nil {
			return err
		}
		if st != nil {
			if st.init {
				// update state since update was successful
				for k, v := range upd {
					st.val[k] = v
				}
			} else {
				st.init = true
				st.val = upd
			}
		}
	}
	return nil
}
