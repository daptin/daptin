package underscore

import (
	"reflect"
)

func Index(source, indexSelector interface{}) interface{} {
	var dictRV reflect.Value
	each(source, indexSelector, func(indexRV, valueRV, _ reflect.Value) bool {
		if !dictRV.IsValid() {
			dictRT := reflect.MapOf(indexRV.Type(), valueRV.Type())
			dictRV = reflect.MakeMap(dictRT)
		}

		dictRV.SetMapIndex(indexRV, valueRV)
		return false
	})
	if dictRV.IsValid() {
		return dictRV.Interface()
	}

	return nil
}

func IndexBy(source interface{}, property string) interface{} {
	getPropertyRV := PropertyRV(property)
	return Index(source, func(value, _ interface{}) Facade {
		rv, _ := getPropertyRV(value)
		return Facade{rv}
	})
}

//Chain
func (this *Query) Index(indexSelector interface{}) Queryer {
	this.source = Index(this.source, indexSelector)
	return this
}

func (this *Query) IndexBy(property string) Queryer {
	this.source = IndexBy(this.source, property)
	return this
}
