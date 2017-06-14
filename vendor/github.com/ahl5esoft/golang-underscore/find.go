package underscore

import (
	"reflect"
)

func Find(source, predicate interface{}) interface{} {
	var ok bool
	var matcher interface{}
	each(source, predicate, func(resRV, valueRV, _ reflect.Value) bool {
		ok = resRV.Bool()
		if ok {
			matcher = valueRV.Interface()
		}
		return ok
	})
	return matcher
}

func FindBy(source interface{}, properties map[string]interface{}) interface{} {
	return Find(source, func(value, _ interface{}) bool {
		return IsMatch(value, properties)
	})
}

//# chain
func (this *Query) Find(predicate interface{}) Queryer {
	this.source = Find(this.source, predicate)
	return this
}

func (this *Query) FindBy(properties map[string]interface{}) Queryer {
	this.source = FindBy(this.source, properties)
	return this
}
