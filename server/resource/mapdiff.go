package resource

import (
	"fmt"
	"reflect"
)

// Diff returns a slice where each element describes
// a difference between a and b.
func Diff(a, b interface{}) (desc []Change) {
	return Fdiff(a, b)
}

// Fdiff writes to w a description of the differences between a and b.
func Fdiff(a, b interface{}) []Change {
	writer := diffWriter{}
	writer.diff(reflect.ValueOf(a), reflect.ValueOf(b))
	return writer.changes
}

type changeType int

const (
	Added        changeType = iota
	Removed
	ChangedType
	ChangedValue
)

type Change struct {
	ChangeType changeType
	OldValue   interface{}
	NewValue   interface{}
}

type diffWriter struct {
	l       string // label
	changes []Change
}

func (w diffWriter) addDiff(diffType changeType, oldValue interface{}, newValue interface{}) {

	w.changes = append(w.changes, Change{
		ChangeType: diffType,
		OldValue:   oldValue,
		NewValue:   newValue,
	})
}

//
//func (w diffWriter) printf(f string, a ...interface{}) {
//  var l string
//  if w.l != "" {
//    l = w.l + ": "
//  }
//  fmt.Fprintf(w.w, l + f, a...)
//}

func (w diffWriter) diff(beforeValue, afterValue reflect.Value) {
	if !beforeValue.IsValid() && afterValue.IsValid() {
		//w.addDiff("nil != %#v", bv.Interface())
		w.addDiff(Added, nil, afterValue.Elem())
		return
	}
	if beforeValue.IsValid() && !afterValue.IsValid() {
		//w.addDiff("%#v != nil", av.Interface())
		w.addDiff(Removed, beforeValue.Elem(), nil)
		return
	}
	if !beforeValue.IsValid() && !afterValue.IsValid() {
		return
	}

	at := beforeValue.Type()
	bt := afterValue.Type()
	if at != bt {
		w.addDiff(ChangedType, at, bt)
		return
	}

	// numeric types, including bool
	if at.Kind() < reflect.Array {
		a, b := beforeValue.Interface(), afterValue.Interface()
		if a != b {
			w.addDiff(ChangedValue, a, b)
		}
		return
	}

	switch at.Kind() {
	case reflect.String:
		a, b := beforeValue.Interface(), afterValue.Interface()
		if a != b {
			w.addDiff(ChangedValue, a, b)
		}
	case reflect.Ptr:
		switch {
		case beforeValue.IsNil() && !afterValue.IsNil():
			w.addDiff(Added, nil, afterValue.Interface())
		case !beforeValue.IsNil() && afterValue.IsNil():
			w.addDiff(Removed, beforeValue.Interface(), nil)
		case !beforeValue.IsNil() && !afterValue.IsNil():
			w.diff(beforeValue.Elem(), afterValue.Elem())
		}
	case reflect.Struct:
		for i := 0; i < beforeValue.NumField(); i++ {
			w.relabel(at.Field(i).Name).diff(beforeValue.Field(i), afterValue.Field(i))
		}
	case reflect.Map:
		ak, both, bk := keyDiff(beforeValue.MapKeys(), afterValue.MapKeys())
		for _, k := range ak {
			w := w.relabel(fmt.Sprintf("[%#v]", k.Interface()))
			//w.printf("%q != (missing)", beforeValue.MapIndex(k))
			w.addDiff(Removed, beforeValue.MapIndex(k), nil)
		}
		for _, k := range both {
			w := w.relabel(fmt.Sprintf("[%#v]", k.Interface()))
			w.diff(beforeValue.MapIndex(k), afterValue.MapIndex(k))
		}
		for _, k := range bk {
			w := w.relabel(fmt.Sprintf("[%#v]", k.Interface()))
			//w.printf("(missing) != %q", afterValue.MapIndex(k))
			w.addDiff(Added, nil, afterValue.MapIndex(k))
		}
	case reflect.Interface:
		w.diff(reflect.ValueOf(beforeValue.Interface()), reflect.ValueOf(afterValue.Interface()))
	default:
		if !reflect.DeepEqual(beforeValue.Interface(), afterValue.Interface()) {
			//w.printf("%# v != %# v", Formatter(beforeValue.Interface()), Formatter(afterValue.Interface()))
			w.addDiff(ChangedValue, beforeValue.Interface(), afterValue.Interface())
		}
	}
}

func (d diffWriter) relabel(name string) (d1 diffWriter) {
	d1 = d
	if d.l != "" && name[0] != '[' {
		d1.l += "."
	}
	d1.l += name
	return d1
}

func keyDiff(a, b []reflect.Value) (ak, both, bk []reflect.Value) {
	for _, av := range a {
		inBoth := false
		for _, bv := range b {
			if reflect.DeepEqual(av.Interface(), bv.Interface()) {
				inBoth = true
				both = append(both, av)
				break
			}
		}
		if !inBoth {
			ak = append(ak, av)
		}
	}
	for _, bv := range b {
		inBoth := false
		for _, av := range a {
			if reflect.DeepEqual(av.Interface(), bv.Interface()) {
				inBoth = true
				break
			}
		}
		if !inBoth {
			bk = append(bk, bv)
		}
	}
	return
}
