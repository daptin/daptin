package underscore

import (
	"reflect"
)

/*
	@source		map or array
	@predicate	func(value or item, key or index) bool
*/
func Any(source, predicate interface{}) bool {
	var ok bool
	each(source, predicate, func(resRV, _, _ reflect.Value) bool {
		ok = resRV.Bool()
		return ok
	})
	return ok
}

func AnyBy(source interface{}, properties map[string]interface{}) bool {
	return Any(source, func(value, _ interface{}) bool {
		return IsMatch(value, properties)
	})
}

//# chain
func (this *Query) Any(predicate interface{}) Queryer {
	this.source = Any(this.source, predicate)
	return this
}

func (this *Query) AnyBy(properties map[string]interface{}) Queryer {
	this.source = AnyBy(this.source, properties)
	return this
}
