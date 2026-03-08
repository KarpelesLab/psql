package psql

import (
	"context"
	"errors"
	"reflect"
)

// Update saves changes to existing database records. Only fields that have changed
// since the last load are updated (if the object was previously fetched). Fires
// [BeforeSaveHook], [BeforeUpdateHook], [AfterUpdateHook], and [AfterSaveHook] if
// implemented. All passed objects must be of the same type.
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

	be := GetBackend(ctx)
	engine := be.Engine()

	for _, obj := range target {
		if h, ok := any(obj).(BeforeSaveHook); ok {
			if err := h.BeforeSave(ctx); err != nil {
				return err
			}
		}
		if h, ok := any(obj).(BeforeUpdateHook); ok {
			if err := h.BeforeUpdate(ctx); err != nil {
				return err
			}
		}

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
		// Get the formatted table name (respects explicit names)
		tableName := t.FormattedName(be)

		d := engine.dialect()
		req := "UPDATE " + QuoteName(tableName) + " SET "
		var flds []any
		first := true
		for k, v := range upd {
			if !first {
				req += ", "
			} else {
				first = false
			}
			flds = append(flds, engine.export(v.v, v.f))
			req += QuoteName(k) + " = " + d.Placeholder(len(flds))
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
			flds = append(flds, engine.export(val.Field(t.fldcol[col].index).Interface(), t.fldcol[col]))
			req += QuoteName(col) + " = " + d.Placeholder(len(flds))
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

		if h, ok := any(obj).(AfterUpdateHook); ok {
			if err := h.AfterUpdate(ctx); err != nil {
				return err
			}
		}
		if h, ok := any(obj).(AfterSaveHook); ok {
			if err := h.AfterSave(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
