package underscore

import (
	"reflect"
)

func filter(source, predicate interface{}, compareValue bool) interface{} {
	var arrRV reflect.Value
	each(source, predicate, func(okRV, valueRV, _ reflect.Value) bool {
		if okRV.Bool() == compareValue {
			if !arrRV.IsValid() {
				arrRT := reflect.SliceOf(valueRV.Type())
				arrRV = reflect.MakeSlice(arrRT, 0, 0)
			}

			arrRV = reflect.Append(arrRV, valueRV)
		}
		return false
	})
	if arrRV.IsValid() {
		return arrRV.Interface()
	}

	return nil
}
