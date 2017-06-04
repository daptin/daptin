package underscore

import (
	"reflect"
)

func Group(source, keySelector interface{}) interface{} {
	var groupRV reflect.Value
	each(source, keySelector, func (groupKeyRV, valueRV, _ reflect.Value) bool {
		groupValueRT := reflect.SliceOf(valueRV.Type())
		if !groupRV.IsValid() {
			groupRT := reflect.MapOf(groupKeyRV.Type(), groupValueRT)
			groupRV = reflect.MakeMap(groupRT)
		}

		valuesRV := groupRV.MapIndex(groupKeyRV)
		if !valuesRV.IsValid() {
			valuesRV = reflect.MakeSlice(groupValueRT, 0, 0)
		}
		valuesRV = reflect.Append(valuesRV, valueRV)
		
		groupRV.SetMapIndex(groupKeyRV, valuesRV)
		return false
	})
	if groupRV.IsValid() {
		return groupRV.Interface()
	}

	return nil
}

func GroupBy(source interface{}, property string) interface{} {
	getPropertyRV := PropertyRV(property)
	return Group(source, func (value, _ interface{}) Facade {
		rv, _ := getPropertyRV(value)
		return Facade{ rv }
	})
}

//Chain
func (this *Query) Group(keySelector interface{}) Queryer {
	this.source = Group(this.source, keySelector)
	return this
}

func (this *Query) GroupBy(property string) Queryer {
	this.source = GroupBy(this.source, property)
	return this
}
