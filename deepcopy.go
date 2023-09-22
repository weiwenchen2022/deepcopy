// package deepcopy makes deep copies of somethings: unexported field values are not copied.
package deepcopy

import (
	"fmt"
	"reflect"
	"sync"
	"time"
)

// DeepClone returns a deep copy of whatever is passed to it and returns the copy
// in an any. The returned value will need to be asserted to the correct type.
//
// DeepClone calls one of methods "Clone() *T" or "Clone() T"
// to delegating copy process to type.
func DeepClone(src any) any {
	srcType := reflect.TypeOf(src)
	if reflect.Pointer == srcType.Kind() {
		srcType = srcType.Elem()
	}

	dst := reflect.New(srcType)
	switch dst.Elem().Kind() {
	case reflect.Slice:
		dst.Elem().Set(reflect.MakeSlice(dst.Type().Elem(), 0, 0))
	case reflect.Map:
		dst.Elem().Set(reflect.MakeMap(dst.Elem().Type()))
	}
	DeepCopy(dst.Interface(), src)

	if timeType == dst.Elem().Type() {
		return dst.Elem().Interface()
	}

	switch dst.Elem().Kind() {
	case reflect.Interface:
		fallthrough
	case reflect.Struct:
		return dst.Interface()
	case reflect.Slice:
		fallthrough
	case reflect.Map:
		fallthrough
	default:
		return dst.Elem().Interface()
	}
}

// DeepCopy copies the contents of src into dst
// See DeepClone function's documentation for more information.
func DeepCopy(dst, src any) {
	dstv := reflect.ValueOf(dst)
	srcv := reflect.ValueOf(src)

	if reflect.Pointer != dstv.Kind() || dstv.IsNil() {
		panic("dst not non-nil pointer")
	}

	if reflect.Pointer == srcv.Kind() && srcv.IsNil() {
		panic("src is nil pointer")
	}

	if reflect.Pointer == srcv.Kind() {
		if dt, st := dstv.Type().Elem(), srcv.Type().Elem(); dt != st {
			panic(fmt.Sprintf("type mistmatch %s != %s", dt, st))
		}
		if srcv.Interface() == dstv.Interface() {
			return
		}
		srcv = srcv.Elem()
	} else if dt, st := dstv.Type().Elem(), srcv.Type(); dt != st {
		panic(fmt.Sprintf("type mistmatch %s != %s", dt, st))
	}

	c, put := newCopyState()
	defer put()
	c.deepValueCopy(dstv.Elem(), srcv)
}

type copyState struct {
	// Keep track of what pointers we've seen in the current recursive call
	// path, to avoid cycles that could lead to a stack overflow. Only do
	// the relatively expensive map operations if ptrLevel is larger than
	// startDetectingCyclesAfter, so that we skip the work if we're within a
	// reasonable amount of nested pointers deep.
	ptrLevel uint
	ptrSeen  map[any]struct{}
}

const startDetectingCyclesAfter = 1000

var copyStatePool sync.Pool

func newCopyState() (c *copyState, put func()) {
	if v := copyStatePool.Get(); v != nil {
		c = v.(*copyState)
		if len(c.ptrSeen) > 0 {
			panic("ptrEncoder.encode should have emptied ptrSeen via defers")
		}
	} else {
		c = &copyState{ptrSeen: make(map[any]struct{})}
	}
	put = func() { copyStatePool.Put(c) }
	return c, put
}

type methodType struct {
	method   reflect.Method
	indirect bool
}

func (m methodType) IsValid() bool {
	return m.method.Func.IsValid()
}

func typeMethod(t reflect.Type) methodType {
	method, ok := t.MethodByName("Clone")
	if !ok {
		return methodType{}
	}

	mt := method.Type
	if mt.NumIn() == 1 && mt.NumOut() == 1 {
		switch retType := mt.Out(0); t {
		case retType:
			fallthrough
		case reflect.PointerTo(retType):
			return methodType{method: method}
		}

		switch retType := mt.Out(0); retType {
		case t:
			fallthrough
		case reflect.PointerTo(t):
			return methodType{method: method}
		}
	}
	return methodType{}
}

var methodCache sync.Map // map[reflect.Type]methodType

// cachedTypeMethod is like typeMethod but uses a cache to avoid repeated work.
func cachedTypeMethod(t reflect.Type) methodType {
	if f, ok := methodCache.Load(t); ok {
		return f.(methodType)
	}

	f, _ := methodCache.LoadOrStore(t, typeMethod(t))
	return f.(methodType)
}

func tryInvokeCloneMethod(dst, src reflect.Value) bool {
	method := cachedTypeMethod(src.Type())
	if method.IsValid() {
		ret := method.method.Func.Call([]reflect.Value{src})[0]

		switch dst.Type() {
		case ret.Type():
		case reflect.PointerTo(ret.Type()):
			newRet := reflect.New(ret.Type()).Elem()
			newRet.Set(ret)
			ret = newRet.Addr()
		}

		switch ret.Type() {
		case dst.Type():
		case reflect.PointerTo(dst.Type()):
			ret = ret.Elem()
		}
		dst.Set(ret)
		return true
	}
	return false
}

var timeType = reflect.TypeOf(time.Time{})

func (c *copyState) deepValueCopy(dst, src reflect.Value) {
	if dst.Type() != src.Type() {
		panic(fmt.Sprintf("type mistmatch %s != %s", dst.Type(), src.Type()))
	}

	if src.Kind() != reflect.Pointer && src.CanAddr() {
		if tryInvokeCloneMethod(dst, src.Addr()) {
			return
		}
	}

	if tryInvokeCloneMethod(dst, src) {
		return
	}

	switch sk := src.Kind(); sk {
	case reflect.Pointer, reflect.Slice, reflect.Map:
		if src.IsNil() {
			dst.SetZero()
			return
		}

		if c.ptrLevel++; c.ptrLevel > startDetectingCyclesAfter {
			var ptr any
			switch sk {
			case reflect.Pointer:
				// We're a large number of nested ptrEncoder.encode calls deep;
				// start checking if we've run into a pointer cycle.
				ptr = src.Interface()

			case reflect.Slice:
				// We're a large number of nested ptrEncoder.encode calls deep;
				// start checking if we've run into a pointer cycle.
				// Here we use a struct to memorize the pointer to the first element of the slice
				// and its length.
				ptr = struct {
					ptr any // always an unsafe.Pointer, but avoids a dependency on package unsafe
					len int
				}{src.UnsafePointer(), src.Len()}

			case reflect.Map:
				// We're a large number of nested ptrEncoder.encode calls deep;
				// start checking if we've run into a pointer cycle.
				ptr = src.UnsafePointer()
			}
			if _, ok := c.ptrSeen[ptr]; ok {
				return
			}
			c.ptrSeen[ptr] = struct{}{}
			defer delete(c.ptrSeen, ptr)
		}
	}

	switch sk := src.Kind(); sk {
	case reflect.Interface:
		if src.IsNil() {
			dst.SetZero()
			return
		}

		srcElem := src.Elem()
		dstElem := reflect.New(srcElem.Type()).Elem()
		c.deepValueCopy(dstElem, srcElem)
		dst.Set(dstElem)

	case reflect.Struct:
		if timeType == src.Type() {
			dst.Set(src)
			return
		}

		for i := 0; i < src.NumField(); i++ {
			if !dst.Field(i).CanSet() {
				continue
			}
			c.deepValueCopy(dst.Field(i), src.Field(i))
		}

	case reflect.Map:
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))

		var mapElem reflect.Value
		for mi := src.MapRange(); mi.Next(); {
			if !mapElem.IsValid() {
				mapElem = reflect.New(src.Type().Elem()).Elem()
			} else {
				mapElem.SetZero()
			}
			c.deepValueCopy(mapElem, mi.Value())
			dst.SetMapIndex(mi.Key(), mapElem)
		}

	case reflect.Slice, reflect.Array:
		if reflect.Slice == sk {
			dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		}

		for i := 0; i < src.Len(); i++ {
			c.deepValueCopy(dst.Index(i), src.Index(i))
		}

	case reflect.Pointer:
		if dst.IsNil() {
			dst.Set(reflect.New(src.Type().Elem()))
		}
		c.deepValueCopy(dst.Elem(), src.Elem())

	default:
		dst.Set(src)
	}
}
