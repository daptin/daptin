package underscore

func Take(source interface{}, count int) interface{} {
	index := 0
	return Select(source, func(_, _ interface{}) bool {
		index = index + 1
		return index <= count
	})
}

//# chain
func (this *Query) Take(count int) Queryer {
	this.source = Take(this.source, count)
	return this
}
