package underscore

func Pluck(source interface{}, property string) interface{} {
	getPropertyRV := PropertyRV(property)
	return Map(source, func(value, _ interface{}) Facade {
		rv, _ := getPropertyRV(value)
		return Facade{rv}
	})
}

//chain
func (this *Query) Pluck(property string) Queryer {
	this.source = Pluck(this.source, property)
	return this
}
