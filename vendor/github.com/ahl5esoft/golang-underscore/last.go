package underscore

func Last(source interface{}) interface{} {
	length, getKeyValue := parseSource(source)
	if length == 0 {
		return nil
	}

	valueRV, _ := getKeyValue(length - 1)
	return valueRV.Interface()
}

//# chain
func (this *Query) Last() Queryer {
	this.source = Last(this.source)
	return this
}
