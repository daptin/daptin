package underscore

import (
	"reflect"
)

func Reduce(source, iterator, memo interface{}) interface{} {
	length, getKeyValue := parseSource(source)
	if length == 0 {
		return memo
	}

	origin := Clone(memo)
	iteratorRV := reflect.ValueOf(iterator)
	memoRV := reflect.ValueOf(memo)
	for i := 0; i < length; i++ {
		valueRV, keyRV := getKeyValue(i)
		returnRVs := iteratorRV.Call(
			[]reflect.Value{
				memoRV,
				valueRV,
				keyRV,
			},
		)
		memoRV = returnRVs[0]
	}

	if memoRV.IsValid() {
		return memoRV.Interface()
	}

	return origin
}

//Chain
func (this *Query) Reduce(iterator, memo interface{}) Queryer {
	this.source = Reduce(this.source, iterator, memo)
	return this
}
