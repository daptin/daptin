package underscore

import (
	"reflect"
)

func Map(source, selector interface{}) interface{} {
	var arrRV reflect.Value
	each(source, selector, func(resRV, valueRV, _ reflect.Value) bool {
		if !arrRV.IsValid() {
			arrRT := reflect.SliceOf(resRV.Type())
			arrRV = reflect.MakeSlice(arrRT, 0, 0)
		}

		arrRV = reflect.Append(arrRV, resRV)
		return false
	})
	if arrRV.IsValid() {
		return arrRV.Interface()
	}

	return nil
}

func MapBy(source interface{}, property string) interface{} {
	getPropertyRV := PropertyRV(property)
	return Map(source, func(value, _ interface{}) Facade {
		rv, _ := getPropertyRV(value)
		return Facade{rv}
	})
}

//chain
func (this *Query) Map(selector interface{}) Queryer {
	this.source = Map(this.source, selector)
	return this
}

func (this *Query) MapBy(property string) Queryer {
	this.source = MapBy(this.source, property)
	return this
}
