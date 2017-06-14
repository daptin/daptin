package underscore

type Queryer interface {
	All(interface{}) Queryer
	AllBy(map[string]interface{}) Queryer
	Any(interface{}) Queryer
	AnyBy(map[string]interface{}) Queryer
	AsParallel() Queryer
	Clone() Queryer
	Each(interface{}) Queryer
	Find(interface{}) Queryer
	FindBy(map[string]interface{}) Queryer
	FindIndex(interface{}) Queryer
	FindIndexBy(map[string]interface{}) Queryer
	First() Queryer
	Group(interface{}) Queryer
	GroupBy(string) Queryer
	Index(interface{}) Queryer
	IndexBy(string) Queryer
	Keys() Queryer
	Last() Queryer
	Map(interface{}) Queryer
	MapBy(string) Queryer
	Pluck(string) Queryer
	Range(int, int, int) Queryer
	Reduce(interface{}, interface{}) Queryer
	Reject(interface{}) Queryer
	RejectBy(map[string]interface{}) Queryer
	Select(interface{}) Queryer
	SelectBy(map[string]interface{}) Queryer
	Size() Queryer
	Sort(interface{}) Queryer
	SortBy(string) Queryer
	Take(int) Queryer
	Uniq(interface{}) Queryer
	UniqBy(string) Queryer
	Value() interface{}
	Values() Queryer
}

type Query struct {
	isParallel bool
	source     interface{}
}

func (this *Query) Value() interface{} {
	return this.source
}

func (this *Query) AsParallel() Queryer {
	this.isParallel = true
	return this
}

func Chain(source interface{}) Queryer {
	q := new(Query)
	q.source = source
	return q
}
