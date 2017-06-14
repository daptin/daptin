package underscore

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

func Md5(plaintext string) string {
	hash := md5.New()
	hash.Write([]byte(plaintext))
	return hex.EncodeToString(hash.Sum(nil))
}

func ParseJson(str string, container interface{}) error {
	reader := strings.NewReader(str)
	return json.NewDecoder(reader).Decode(container)
}

func ToJson(value interface{}) (string, error) {
	var err error
	res := ""

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String:
		res = value.(string)
		break
	case reflect.Array,
		reflect.Map,
		reflect.Slice,
		reflect.Struct:
		var bytes []uint8
		bytes, err = json.Marshal(value)
		if err == nil {
			res = string(bytes)
		}
		break
	case reflect.Bool:
		res = strconv.FormatBool(value.(bool))
		break
	case reflect.Float32, reflect.Float64:
		res = strconv.FormatFloat(
			rv.Float(),
			'f',
			-1,
			64,
		)
		break
	case reflect.Int,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Int8:
		res = strconv.FormatInt(
			rv.Int(),
			10,
		)
		break
	case reflect.Ptr:
		res, err = ToJson(
			reflect.Indirect(rv).Interface(),
		)
		break
	case reflect.Uint,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uint8:
		res = strconv.FormatUint(
			rv.Uint(),
			10,
		)
		break
	}
	return res, err
}

func ToRealValue(rv reflect.Value) interface{} {
	var value interface{}
	switch rv.Kind() {
	case reflect.Bool:
		value = rv.Bool()
		break
	case reflect.Float32, reflect.Float64:
		value = rv.Float()
		break
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		value = rv.Int()
		break
	case reflect.String:
		value = rv.String()
		break
	case reflect.Struct:
		value = rv.Interface()
		break
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value = rv.Uint()
		break
	case reflect.Ptr:
		return ToRealValue(
			reflect.Indirect(rv),
		)
	default:
		if !rv.IsNil() {
			value = rv.Interface()
		}
		break
	}
	return value
}

func UUID() string {
	uuid := make([]byte, 16)
	n, err := rand.Read(uuid)
	if n != len(uuid) || err != nil {
		return UUID()
	}
	uuid[8] = 0x80
	uuid[4] = 0x40
	return hex.EncodeToString(uuid)
}
