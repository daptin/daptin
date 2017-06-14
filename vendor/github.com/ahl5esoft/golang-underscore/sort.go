package underscore

import (
	"reflect"
	"sort"
)

type sortQuery struct {
	keysRV    reflect.Value
	valuesRV  reflect.Value
	compareRV reflect.Value
}

func (this sortQuery) Len() int {
	if this.keysRV.IsValid() {
		return this.keysRV.Len()
	}

	return 0
}

func (this sortQuery) Swap(i, j int) {
	temp := this.keysRV.Index(i).Interface()
	this.keysRV.Index(i).Set(
		this.keysRV.Index(j),
	)
	this.keysRV.Index(j).Set(
		reflect.ValueOf(temp),
	)

	temp = this.valuesRV.Index(i).Interface()
	this.valuesRV.Index(i).Set(
		this.valuesRV.Index(j),
	)
	this.valuesRV.Index(j).Set(
		reflect.ValueOf(temp),
	)
}

func (this sortQuery) Less(i, j int) bool {
	thisRV := this.keysRV.Index(i)
	thatRV := this.keysRV.Index(j)
	switch thisRV.Kind() {
	case reflect.Float32, reflect.Float64:
		return thisRV.Float() < thatRV.Float()
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		return thisRV.Int() < thatRV.Int()
	case reflect.String:
		return thisRV.String() < thatRV.String()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return thisRV.Uint() < thatRV.Uint()
	default:
		return false
	}
}

func Sort(source, selector interface{}) interface{} {
	qs := sortQuery{}
	each(source, selector, func(sortRV, valueRV, _ reflect.Value) bool {
		if qs.Len() == 0 {
			keysRT := reflect.SliceOf(sortRV.Type())
			qs.keysRV = reflect.MakeSlice(keysRT, 0, 0)

			valuesRT := reflect.SliceOf(valueRV.Type())
			qs.valuesRV = reflect.MakeSlice(valuesRT, 0, 0)
		}

		qs.keysRV = reflect.Append(qs.keysRV, sortRV)
		qs.valuesRV = reflect.Append(qs.valuesRV, valueRV)
		return false
	})
	if qs.Len() > 0 {
		sort.Sort(qs)
		return qs.valuesRV.Interface()
	}

	return nil
}

func SortBy(source interface{}, property string) interface{} {
	getPropertyRV := PropertyRV(property)
	return Sort(source, func(value, _ interface{}) Facade {
		rv, _ := getPropertyRV(value)
		return Facade{rv}
	})
}

//chain
func (this *Query) Sort(selector interface{}) Queryer {
	this.source = Sort(this.source, selector)
	return this
}

func (this *Query) SortBy(property string) Queryer {
	this.source = SortBy(this.source, property)
	return this
}
