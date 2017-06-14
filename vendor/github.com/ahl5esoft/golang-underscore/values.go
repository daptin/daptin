package underscore

import (
	"reflect"
)

func Values(source interface{}) interface{} {
	sourceRV := reflect.ValueOf(source)
	if sourceRV.Kind() != reflect.Map {
		return nil
	}

	return Map(source, func(value, _ interface{}) Facade {
		return Facade{reflect.ValueOf(value)}
	})
}

//Chain
func (this *Query) Values() Queryer {
	this.source = Values(this.source)
	return this
}
