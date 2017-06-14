package underscore

import (
	"reflect"
)

func FindIndex(source, predicate interface{}) int {
	index := -1

	if !IsArray(source) {
		return index
	}

	each(source, predicate, func(okRV, _, keyRV reflect.Value) bool {
		ok := okRV.Bool()
		if ok {
			index = int(keyRV.Int())
		}
		return ok
	})

	return index
}

func FindIndexBy(source interface{}, properties map[string]interface{}) int {
	return FindIndex(source, func(item interface{}, _ int) bool {
		return IsMatch(item, properties)
	})
}

//# chain
func (this *Query) FindIndex(predicate interface{}) Queryer {
	this.source = FindIndex(this.source, predicate)
	return this
}

func (this *Query) FindIndexBy(properties map[string]interface{}) Queryer {
	this.source = FindIndexBy(this.source, properties)
	return this
}
