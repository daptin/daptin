package underscore

func First(source interface{}) interface{} {
	length, getKeyValue := parseSource(source)
	if length == 0 {
		return nil
	}

	valueRV, _ := getKeyValue(0)
	return valueRV.Interface()
}

//# chain
func (this *Query) First() Queryer {
	this.source = First(this.source)
	return this
}
