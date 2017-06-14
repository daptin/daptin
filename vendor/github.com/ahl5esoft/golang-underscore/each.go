package underscore

import (
	"reflect"
	"sync"
)

func Each(source, iterator interface{}) {
	each(source, iterator, nil)
}

/*
	@source		数据源,array or map
	@iterator	迭代器
	@predicate	值判断
		@in1		iterator结果值
		@in2		value(reflect.Value)
		@in3		key(reflect.Value)
		@out		是否break
*/
func each(source interface{}, iterator interface{}, predicate func(reflect.Value, reflect.Value, reflect.Value) bool) {
	length, getKeyValue := parseSource(source)
	if length == 0 {
		return
	}

	if predicate == nil {
		predicate = func(resRV, _, _ reflect.Value) bool {
			if resRV.Kind() == reflect.Bool {
				return resRV.Bool()
			} else {
				return false
			}
		}
	}

	iteratorRV := reflect.ValueOf(iterator)
	for i := 0; i < length; i++ {
		valueRV, keyRV := getKeyValue(i)
		returnRVs := iteratorRV.Call(
			[]reflect.Value{valueRV, keyRV},
		)
		if len(returnRVs) > 0 {
			resRV := returnRVs[0]
			if resRV.Type() == FACADE_RT {
				resRV = resRV.Interface().(Facade).Real
			}

			if predicate(resRV, valueRV, keyRV) {
				break
			}
		}
	}
}

func eachAsParallel(source interface{}, iterator interface{}) {
	length, getKeyValue := parseSource(source)
	if length == 0 {
		return
	}

	var task sync.WaitGroup
	task.Add(length)

	iteratorRV := reflect.ValueOf(iterator)
	for i := 0; i < length; i++ {
		go func(index int) {
			valueRV, keyRV := getKeyValue(index)
			iteratorRV.Call(
				[]reflect.Value{valueRV, keyRV},
			)

			task.Done()
		}(i)
	}

	task.Wait()
}

func parseSource(source interface{}) (int, func(i int) (reflect.Value, reflect.Value)) {
	if source != nil {
		sourceRV := reflect.ValueOf(source)
		switch sourceRV.Kind() {
		case reflect.Array:
		case reflect.Slice:
			return sourceRV.Len(), func(i int) (reflect.Value, reflect.Value) {
				return sourceRV.Index(i), reflect.ValueOf(i)
			}
		case reflect.Map:
			keyRVs := sourceRV.MapKeys()
			return len(keyRVs), func(i int) (reflect.Value, reflect.Value) {
				return sourceRV.MapIndex(keyRVs[i]), keyRVs[i]
			}
		}
	}
	return 0, nil
}

//Chain
func (this *Query) Each(iterator interface{}) Queryer {
	if this.isParallel {
		eachAsParallel(this.source, iterator)
	} else {
		each(this.source, iterator, nil)
	}

	return this
}
