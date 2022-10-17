package psql

import (
	"reflect"
)

func dup(v []byte) []byte {
	r := make([]byte, len(v))
	copy(r, v)
	return r
}

func dupv[T any](v T) T {
	return dupr(reflect.ValueOf(v)).Interface().(T)
}

func dupr(src reflect.Value) reflect.Value {
	if !src.IsValid() {
		// wut?
		return src
	}

	switch src.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32,
		reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32,
		reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64,
		reflect.Complex64, reflect.Complex128, reflect.Func:
		// can be used as is
		return src
	case reflect.String:
		// strings are not writable
		return src
	case reflect.Slice, reflect.Array:
		size := src.Len()
		dst := reflect.New(src.Type()).Elem()
		if src.Kind() == reflect.Slice {
			dst.SetCap(size)
			dst.SetLen(size)
		}
		for i := 0; i < size; i++ {
			dst.Index(i).Set(dupr(src.Index(i)))
		}
		return dst
	case reflect.Map:
		dst := reflect.New(src.Type()).Elem()
		iter := src.MapRange()
		for iter.Next() {
			dst.SetMapIndex(dupr(iter.Key()), dupr(iter.Value()))
		}
		return dst
	case reflect.Ptr, reflect.Interface:
		newPtr := reflect.New(src.Type()).Elem()
		if !src.IsNil() {
			newPtr.Set(dupr(src.Elem()).Addr())
		}
		return newPtr
	case reflect.Struct:
		dst := reflect.New(src.Type()).Elem()
		n := src.NumField()
		for i := 0; i < n; i += 1 {
			if !src.Type().Field(i).IsExported() {
				continue
			}
			dst.Field(i).Set(dupr(src.Field(i)))
			// We do not care about unexported fields since we don't handle these in mysql either
			//log.Printf("type = %s", dst.Field(i).Type())
			//field := dst.Field(i)
			//val := dupr(reflect.NewAt(field.Type(), unsafe.Pointer(src.Field(i).UnsafeAddr())).Elem())
			//reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem().Set(val)
		}
		return dst
	case reflect.UnsafePointer:
		fallthrough
	default:
		dst := reflect.New(src.Type()).Elem()
		dst.Set(src)
		return dst
	}
}
