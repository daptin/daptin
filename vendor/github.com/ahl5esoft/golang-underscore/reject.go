package underscore

func Reject(source, predicate interface{}) interface{} {
	return filter(source, predicate, false)
}

func RejectBy(source interface{}, properties map[string]interface{}) interface{} {
	return Reject(source, func(value, _ interface{}) bool {
		return IsMatch(value, properties)
	})
}

//# chain
func (this *Query) Reject(predicate interface{}) Queryer {
	this.source = Reject(this.source, predicate)
	return this
}

func (this *Query) RejectBy(properties map[string]interface{}) Queryer {
	this.source = RejectBy(this.source, properties)
	return this
}
