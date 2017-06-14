package underscore

import (
	"reflect"
)

func Clone(source interface{}) interface{} {
	rv := reflect.ValueOf(source)
	switch rv.Kind() {
	case reflect.Array:
	case reflect.Slice:
		dstRV := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		if rv.Len() != 0 {
			for i := 0; i < rv.Len(); i++ {
				dstRV.Index(i).Set(
					rv.Index(i),
				)
			}
		}

		return dstRV.Interface()
	case reflect.Map:
		dstRV := reflect.MakeMap(rv.Type())

		keyRVs := rv.MapKeys()
		if len(keyRVs) != 0 {
			for i := 0; i < len(keyRVs); i++ {
				dstRV.SetMapIndex(
					keyRVs[i],
					rv.MapIndex(keyRVs[i]),
				)
			}
		}
		return dstRV.Interface()
	case reflect.Ptr:
		return Clone(reflect.Indirect(rv).Interface())
	}
	return source
}

//Chain
func (this *Query) Clone() Queryer {
	this.source = Clone(this.source)
	return this
}
