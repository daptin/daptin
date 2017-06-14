package underscore

import (
	"errors"
	"reflect"
)

/*
	包装真实的值
	each的时候 如果iterator返回Facade 则会将Real替换iterator返回值
*/
type Facade struct {
	Real reflect.Value
}

var (
	ERROR_RT  = reflect.TypeOf(errors.New(""))
	FACADE_RT = reflect.TypeOf(Facade{})
	NULL_RV   = reflect.ValueOf(nil)
	RV_RT     = reflect.TypeOf(NULL_RV)
)
