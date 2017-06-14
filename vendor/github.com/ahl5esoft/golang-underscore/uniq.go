package underscore

import (
	"reflect"
)

func Uniq(source, selector interface{}) interface{} {
	if selector == nil {
		selector = func(value, _ interface{}) Facade {
			return Facade{reflect.ValueOf(value)}
		}
	}

	var mapRV reflect.Value
	var arrRV reflect.Value
	each(source, selector, func(resRV, valueRv, _ reflect.Value) bool {
		if !mapRV.IsValid() {
			mapRT := reflect.MapOf(resRV.Type(), reflect.TypeOf(false))
			mapRV = reflect.MakeMap(mapRT)

			arrRT := reflect.SliceOf(valueRv.Type())
			arrRV = reflect.MakeSlice(arrRT, 0, 0)
		}

		mapValueRV := mapRV.MapIndex(resRV)
		if !mapValueRV.IsValid() {
			mapRV.SetMapIndex(resRV, reflect.ValueOf(true))
			arrRV = reflect.Append(arrRV, valueRv)
		}
		return false
	})
	if mapRV.IsValid() {
		return arrRV.Interface()
	}

	return nil
}

func UniqBy(source interface{}, property string) interface{} {
	getProeprtyRV := PropertyRV(property)
	return Uniq(source, func(value, _ interface{}) Facade {
		rv, _ := getProeprtyRV(value)
		return Facade{rv}
	})
}

//chain
func (this *Query) Uniq(selector interface{}) Queryer {
	this.source = Uniq(this.source, selector)
	return this
}

func (this *Query) UniqBy(property string) Queryer {
	this.source = UniqBy(this.source, property)
	return this
}
