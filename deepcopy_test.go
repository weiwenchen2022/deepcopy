package deepcopy

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestBasic(t *testing.T) {
	t.Parallel()

	type mystring string

	one := 1
	var iface1 any = &one

	tests := []any{
		1,
		1.0,
		"foo",
		mystring("foo"),

		[]string{"foo", "bar"},
		[]string{},
		[]string(nil),

		map[string][]int{"foo": {1, 2, 3}},
		map[string][]int{},
		map[string][]int(nil),

		[...]int{1, 2, 3},
		&iface1,
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("DeepClone(%#v)", test), func(t *testing.T) {
			testc := DeepClone(test)
			equal(t, test, testc)
		})
	}
}

func TestSlice(t *testing.T) {
	t.Parallel()

	tests := []any{
		[]int{1, 2, 3},
		[]string{"foo", "bar"},
		[]string(nil),
		[]string{},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("DeepClone(%#v)", test), func(t *testing.T) {
			testc := DeepClone(test)
			equal(t, test, testc)
		})
	}
}

func TestMap(t *testing.T) {
	t.Parallel()

	tests := []any{
		map[string][]int{"foo": {1, 2, 3}},
		map[string][]int{},
		map[string][]int(nil),
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("DeepClone(%#v)", test), func(t *testing.T) {
			testc := DeepClone(test)
			equal(t, test, testc)
		})
	}
}

func TestStruct(t *testing.T) {
	t.Parallel()

	type mystring string
	type S1 struct {
		Ints []int
	}
	type S2 struct {
		I        int
		F        float64
		String   string
		MyString mystring

		Strings    []string
		EmptySlice []string
		NilSlice   []string

		Map      map[string][]int
		EmptyMap map[string][]int
		NilMap   map[string][]int

		StructSlice        []S1
		StructPointerSlice []*S1

		StructMap        map[string]S1
		StructPointerMap map[string]*S1

		S1
		T time.Time
	}

	var src = &S2{
		I: 1,
		F: 1.0,

		String:   "foo",
		MyString: mystring("foo"),

		Strings:    []string{"foo", "bar"},
		EmptySlice: []string{},

		Map:      map[string][]int{"foo": {1, 2, 3}},
		EmptyMap: map[string][]int{},

		StructSlice:        []S1{{[]int{1, 2, 3}}},
		StructPointerSlice: []*S1{{[]int{1, 2, 3}}},

		StructMap: map[string]S1{
			"foo": {[]int{1, 2, 3}},
		},
		StructPointerMap: map[string]*S1{
			"foo": {[]int{1, 2, 3}},
		},

		S1: S1{[]int{1, 2, 3}},
		T:  time.Now(),
	}

	dst := DeepClone(src).(*S2)
	equal(t, src, dst)
}

func TestUnexportedFields(t *testing.T) {
	t.Parallel()

	type Unexported struct {
		a string
		b int
		c []int
		d map[string]string
	}
	src := &Unexported{
		a: "foobar",
		b: 23,
		c: []int{23},
		d: map[string]string{"foo": "bar"},
	}
	dst := DeepClone(src).(*Unexported)
	if dst == src {
		t.Fatal("expected different pointer")
	}
	if !cmp.Equal(&Unexported{}, dst, cmp.AllowUnexported(Unexported{})) {
		t.Error(cmp.Diff(&Unexported{}, dst, cmp.AllowUnexported(Unexported{})))
	}
}

func TestTimeCopy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		year                      int
		month                     time.Month
		day, hour, min, sec, nsec int
		TZ                        string
	}{
		{2016, time.July, 4, 23, 11, 33, 3000, "America/New_York"},
		{2015, time.October, 31, 9, 44, 23, 45935, "UTC"},
		{2014, time.May, 5, 22, 01, 50, 219300, "Europe/Prague"},
	}

	for i, tt := range tests {
		loc, err := time.LoadLocation(tt.TZ)
		if err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
			continue
		}

		src := time.Date(tt.year, tt.month, tt.day, tt.hour, tt.min, tt.sec, tt.nsec, loc)
		dst := DeepClone(src).(time.Time)
		if !src.Equal(dst) {
			t.Error("time copy error")
		}
	}
}

type Bar struct {
	A string
}

const text = "foobar"

func (*Bar) Clone() *Bar {
	return &Bar{text}
}

type Foo struct {
	*Bar
}

func TestClone(t *testing.T) {
	t.Parallel()

	bar := &Bar{"hello"}
	bc := DeepClone(bar).(*Bar)
	if text != bc.A {
		t.Errorf("got %q, want %q", bc.A, text)
	}

	foo := &Foo{&Bar{"hello"}}
	fc := DeepClone(foo).(*Foo)
	if text != fc.A {
		t.Errorf("got %q, want %q", fc.A, text)
	}
}

func equal(t testing.TB, x, y any, opts ...cmp.Option) bool {
	t.Helper()

	if x == nil || y == nil {
		if x != y {
			t.Errorf("%#v != %#v", x, y)
			return false
		}
		return true
	}

	v1 := reflect.ValueOf(x)
	v2 := reflect.ValueOf(y)
	if v1.Type() != v2.Type() {
		t.Errorf("%s != %s", v1.Type(), v2.Type())
		return false
	}
	return valueEqual(t, v1, v2, opts...)
}

func valueEqual(t testing.TB, v1, v2 reflect.Value, opts ...cmp.Option) bool {
	t.Helper()

	if !v1.IsValid() || !v2.IsValid() {
		if v1.IsValid() != v2.IsValid() {
			t.Errorf("IsValid mismatch %t != %t", v1.IsValid(), v2.IsValid())
			return false
		}
		return true
	}
	if v1.Type() != v2.Type() {
		t.Errorf("%s != %s", v1.Type(), v2.Type())
		return false
	}

	switch v1.Kind() {
	case reflect.Slice:
		if v1.Len() == 0 || v2.Len() == 0 {
			if v1.IsNil() != v2.IsNil() {
				t.Errorf("nil, mismatch %t != %t", v1.IsNil(), v2.IsNil())
				return false
			}
			return true
		}

		if v1.Len() != v2.Len() {
			t.Errorf("len, mismatch %d != %d", v1.Len(), v2.Len())
			return false
		}
		if v1.UnsafePointer() == v2.UnsafePointer() {
			t.Errorf("pointer to same underlying array %p == %p", v1.UnsafePointer(), v2.UnsafePointer())
			return false
		}

		for i := 0; i < v1.Len(); i++ {
			if !valueEqual(t, v1.Index(i), v2.Index(i), opts...) {
				return false
			}
		}
		return true
	case reflect.Interface:
		if v1.IsNil() || v2.IsNil() {
			if v1.IsNil() != v2.IsNil() {
				t.Errorf("nil, mismatch %t != %t", v1.IsNil(), v2.IsNil())
				return false
			}
			return true
		}
		return valueEqual(t, v1.Elem(), v2.Elem(), opts...)
	case reflect.Pointer:
		if v1.IsNil() || v2.IsNil() {
			if v1.IsNil() != v2.IsNil() {
				t.Errorf("nil, mismatch %t != %t", v1.IsNil(), v2.IsNil())
				return false
			}
			return true
		}
		if v1.UnsafePointer() == v2.UnsafePointer() {
			t.Errorf("pointer to same location %p == %p", v1.UnsafePointer(), v2.UnsafePointer())
			return false
		}
		return valueEqual(t, v1.Elem(), v2.Elem(), opts...)

	case reflect.Struct:
		if timeType == v1.Type() {
			return v1.Interface().(time.Time).Equal(v2.Interface().(time.Time))
		}

		for i := 0; i < v1.NumField(); i++ {
			if !valueEqual(t, v1.Field(i), v2.Field(i), opts...) {
				t.Errorf("%s %s mismatch", v1.Type().Field(i).Name, v1.Type())
				return false
			}
		}
		return true

	case reflect.Map:
		if v1.Len() == 0 || v2.Len() == 0 {
			if v1.IsNil() != v2.IsNil() {
				t.Errorf("nil, mismatch %t != %t", v1.IsNil(), v2.IsNil())
				return false
			}
			return true
		}

		if v1.Len() != v2.Len() {
			t.Errorf("len, mismatch %d != %d", v1.Len(), v2.Len())
			return false
		}
		if v1.UnsafePointer() == v2.UnsafePointer() {
			t.Errorf("pointer to same location")
			return false
		}

		for mi := v1.MapRange(); mi.Next(); {
			val1 := mi.Value()
			val2 := v2.MapIndex(mi.Key())
			if !val1.IsValid() || !val2.IsValid() || !valueEqual(t, val1, val2, opts...) {
				return false
			}
		}
		return true

	default:
		if !cmp.Equal(v1.Interface(), v2.Interface(), opts...) {
			t.Error(cmp.Diff(v1.Interface(), v2.Interface(), opts...))
			return false
		}
	}

	return true
}
