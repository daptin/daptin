package underscore

import (
	"reflect"
)

func Keys(source interface{}) interface{} {
	sourceRV := reflect.ValueOf(source)
	if sourceRV.Kind() != reflect.Map {
		return nil
	}

	return Map(source, func(_, key interface{}) Facade {
		return Facade{reflect.ValueOf(key)}
	})
}

//Chain
func (this *Query) Keys() Queryer {
	this.source = Keys(this.source)
	return this
}
