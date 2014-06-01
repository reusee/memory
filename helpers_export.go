package main

/*
#include <glib-object.h>
*/
import "C"
import (
	"fmt"
	"reflect"
	"unsafe"
)

//export closureMarshal
func closureMarshal(closure *C.GClosure, ret *C.GValue, nParams C.guint, params *C.GValue, hint, data C.gpointer) {
	// callback value
	f := *((*interface{})(unsafe.Pointer(data)))
	fValue := reflect.ValueOf(f)
	fType := fValue.Type()

	// convert GValue to reflect.Value
	var paramSlice []C.GValue
	h := (*reflect.SliceHeader)(unsafe.Pointer(&paramSlice))
	h.Len = int(nParams)
	h.Cap = h.Len
	h.Data = uintptr(unsafe.Pointer(params))
	var arguments []reflect.Value
	for i, gv := range paramSlice {
		if i == fType.NumIn() {
			break
		}
		arguments = append(arguments, reflect.ValueOf(fromGValue(&gv)))
	}

	// call
	r := fValue.Call(arguments[:fType.NumIn()])

	// return
	if len(r) > 0 {
		switch r[0].Type().Kind() {
		case reflect.Ptr:
			C.g_value_set_object(ret, (C.gpointer)(unsafe.Pointer(r[0].Pointer())))
		default:
			panic(fmt.Sprintf("unknown return type %v", r[0].Type()))
		}
	}

}

//export callIdleCb
func callIdleCb(fp unsafe.Pointer) {
	f := *(*func())(fp)
	f()
}
