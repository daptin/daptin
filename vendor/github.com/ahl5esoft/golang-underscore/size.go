package underscore

func Size(source interface{}) int {
	length, _ := parseSource(source)
	return length
}

//chain
func (this *Query) Size() Queryer {
	this.source = Size(this.source)
	return this
}
