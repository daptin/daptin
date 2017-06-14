package underscore

func Select(source, predicate interface{}) interface{} {
	return filter(source, predicate, true)
}

func SelectBy(source interface{}, properties map[string]interface{}) interface{} {
	return Select(source, func(value, _ interface{}) bool {
		return IsMatch(value, properties)
	})
}

//# chain
func (this *Query) Select(predicate interface{}) Queryer {
	this.source = Select(this.source, predicate)
	return this
}

func (this *Query) SelectBy(properties map[string]interface{}) Queryer {
	this.source = SelectBy(this.source, properties)
	return this
}
