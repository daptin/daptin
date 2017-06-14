package underscore

import (
	"reflect"
)

/*
	@source		map or array
	@predicate	func(value or item, key or index) bool
*/
func All(source, predicate interface{}) bool {
	var ok bool
	each(source, predicate, func(resRV, _, _ reflect.Value) bool {
		ok = resRV.Bool()
		return !ok
	})
	return ok
}

func AllBy(source interface{}, properties map[string]interface{}) bool {
	return All(source, func(value, _ interface{}) bool {
		return IsMatch(value, properties)
	})
}

//# chain
func (this *Query) All(predicate interface{}) Queryer {
	this.source = All(this.source, predicate)
	return this
}

func (this *Query) AllBy(properties map[string]interface{}) Queryer {
	this.source = AllBy(this.source, properties)
	return this
}
