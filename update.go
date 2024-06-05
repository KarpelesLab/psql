package psql

import (
	"context"
	"errors"
	"reflect"
	"strconv"
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

type updatedField struct {
	f *structField
	v any
}

func (t *TableMeta[T]) Update(ctx context.Context, target ...*T) error {
	if t == nil {
		return ErrNotReady
	}
	t.check(ctx)
	if t.mainKey == nil {
		return errors.New("cannot update values without a unique key")
	}

	engine := GetBackend(ctx).Engine()

	for _, obj := range target {
		// check for changed values
		upd := make(map[string]*updatedField)
		allvals := make(map[string]any)

		val := reflect.ValueOf(obj).Elem()

		st := t.rowstate(obj)
		if st == nil || !st.init {
			// we don't have a state → update everything
			for _, f := range t.fields {
				v := val.Field(f.index).Interface()
				upd[f.column] = &updatedField{f: f, v: v}
				allvals[f.column] = v
			}
		} else {
			for _, f := range t.fields {
				// grab state value
				stv, ok := st.val[f.column]
				newv := val.Field(f.index).Interface()
				allvals[f.column] = newv

				if !ok {
					// no value in state → just force update
					upd[f.column] = &updatedField{f: f, v: newv}
					continue
				}
				if !reflect.DeepEqual(newv, stv) {
					upd[f.column] = &updatedField{f: f, v: newv}
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
			switch engine {
			case EnginePostgreSQL:
				// need to add $1, $2, $3, ...
				req += QuoteName(k) + " = $" + strconv.FormatUint(uint64(len(flds))+1, 10)
			case EngineMySQL:
				fallthrough
			default:
				req += QuoteName(k) + " = ?"
			}
			flds = append(flds, engine.export(v.v, v.f))
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

			switch engine {
			case EnginePostgreSQL:
				// need to add $1, $2, $3, ...
				req += QuoteName(col) + " = $" + strconv.FormatUint(uint64(len(flds))+1, 10)
			case EngineMySQL:
				fallthrough
			default:
				req += QuoteName(col) + " = ?"
			}
			flds = append(flds, engine.export(val.Field(t.fldcol[col].index).Interface(), t.fldcol[col]))
		}

		_, err := ExecContext(ctx, req, flds...)
		if err != nil {
			return err
		}
		if st != nil {
			if st.init {
				// update state since update was successful
				for k, v := range upd {
					st.val[k] = v.v
				}
			} else {
				st.init = true
				st.val = allvals
			}
		}
	}
	return nil
}
