package underscore

import (
	"reflect"
)

func IsArray(v interface{}) bool {
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Array || rv.Kind() == reflect.Slice
}
