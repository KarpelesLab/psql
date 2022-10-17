package psql

import (
	"reflect"
	"unsafe"
)

type rowState struct {
	init bool
	val  map[string]any
}

type stateIntf interface {
	state() *rowState
}

func (t *TableMeta[T]) rowstate(v *T) *rowState {
	if t.state == -1 {
		return nil
	}

	val := reflect.ValueOf(v).Elem().Field(t.state)
	// grab value for pointer
	rf := reflect.NewAt(val.Type(), unsafe.Pointer(val.UnsafeAddr()))

	return rf.Interface().(stateIntf).state()
}
