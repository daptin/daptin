package main

import (
	"fmt"
	"net/url"
	"reflect"
	"testing"
)

func TestForm(t *testing.T) {
	val := ToURLQuery(map[string]interface{}{
		"key1": "val1",
		"key2": map[string]interface{}{
			"key3": "val3",
			"key4": map[string]interface{}{
				"key5": "val5",
			},
		},
	})
	fmt.Printf("TestForm: %s", val)
}

// encodeQuery is a recursive function that generates URL-encoded query strings
func encodeQuery(key string, value interface{}, v url.Values) {
	rv := reflect.ValueOf(value)

	switch rv.Kind() {
	case reflect.Map:
		for _, k := range rv.MapKeys() {
			mapKey := fmt.Sprintf("%v", k)
			encodeQuery(fmt.Sprintf("%s[%s]", key, mapKey), rv.MapIndex(k).Interface(), v)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			encodeQuery(fmt.Sprintf("%s[%d]", key, i), rv.Index(i).Interface(), v)
		}
	case reflect.Struct:
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			fieldName := field.Name
			fieldValue := rv.Field(i).Interface()
			encodeQuery(fmt.Sprintf("%s.%s", key, fieldName), fieldValue, v)
		}
	default:
		v.Add(key, fmt.Sprintf("%v", value))
	}
}

// ToURLQuery converts a Go object into a x-www-form-urlencoded query string
func ToURLQuery(input interface{}) string {
	v := url.Values{}
	rv := reflect.ValueOf(input)

	if rv.Kind() == reflect.Map {
		for _, k := range rv.MapKeys() {
			mapKey := fmt.Sprintf("%v", k)
			encodeQuery(mapKey, rv.MapIndex(k).Interface(), v)
		}
	} else if rv.Kind() == reflect.Struct {
		for i := 0; i < rv.NumField(); i++ {
			field := rv.Type().Field(i)
			fieldName := field.Name
			fieldValue := rv.Field(i).Interface()
			encodeQuery(fieldName, fieldValue, v)
		}
	} else {
		encodeQuery("", input, v)
	}

	return v.Encode()
}
