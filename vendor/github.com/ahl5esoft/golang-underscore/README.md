```                __
                  /\ \                                                       
 __  __    ___    \_\ \     __   _ __   ____    ___    ___   _ __    __	         __     ___
/\ \/\ \ /' _ `\  /'_  \  /'__`\/\  __\/ ,__\  / ___\ / __`\/\  __\/'__`\      /'_ `\  / __`\
\ \ \_\ \/\ \/\ \/\ \ \ \/\  __/\ \ \//\__, `\/\ \__//\ \ \ \ \ \//\  __/  __ /\ \L\ \/\ \L\ \
 \ \____/\ \_\ \_\ \___,_\ \____\\ \_\\/\____/\ \____\ \____/\ \_\\ \____\/\_\\ \____ \ \____/
  \/___/  \/_/\/_/\/__,_ /\/____/ \/_/ \/___/  \/____/\/___/  \/_/ \/____/\/_/ \/___L\ \/___/
                                                                                 /\____/
                                                                                 \_/__/
```

Underscore.go
==========================================

like <a href="http://underscorejs.org/">underscore.js</a>, but for Go

## Installation

    $ go get github.com/ahl5esoft/golang-underscore

## Update
	$ go get -u github.com/ahl5esoft/golang-underscore

## Lack
	* FindLastIndex
	* Sample
	* more...

## Documentation

### API
* [`All`](#all), [`AllBy`](#allBy)
* [`Any`](#any), [`AnyBy`](#anyBy)
* [`AsParallel`](#asParallel)
* [`Chain`](#chain)
* [`Clone`](#clone)
* [`Each`](#each)
* [`Find`](#find), [`FindBy`](#findBy)
* [`FindIndex`](#findIndex), [`FindIndexBy`](#findIndexBy)
* [`First`](#first)
* [`Group`](#group), [`GroupBy`](#groupBy)
* [`IsArray`](#isArray)
* [`IsMatch`](#isMatch)
* [`Keys`](#keys)
* [`Map`](#map), [`MapBy`](#mapBy)
* [`Md5`](#md5)
* [`ParseJson`](#parseJson)
* [`Pluck`](#pluck)
* [`Property`](#property), [`PropertyRV`](#propertyRV)
* [`Range`](#range)
* [`Reduce`](#reduce)
* [`Reject`](#reject), [`RejectBy`](#rejectBy)
* [`Select`](#select), [`SelectBy`](#selectBy)
* [`Size`](#size)
* [`Sort`](#sort), [`SortBy`](#sortBy)
* [`Take`](#take)
* [`ToJson`](#toJson)
* [`Uniq`](#uniq), [`UniqBy`](#uniqBy)
* [`UUID`](#uuid)
* [`Values`](#values)

<a name="all" />
### All(source, predicate)

__Arguments__

* `source` - array or map
* `predicate` - func(element, index or key) bool

__Return__

* bool - all the values that pass a truth test `predicate`

__Examples__

```go
arr := []int{ 2, 4 }
ok := All(arr, func (n, _ int) bool {
	return n % 2 == 0
})
if !ok {
	// wrong
}
```

<a name="allBy" />
### AllBy(source, properties)

__Arguments__

* `source` - array or map
* `properties` - map[string]interface{}

__Return__

* bool

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
	TestModel{ 3, "three" },
}
ok := AllBy(arr, map[string]interface{}{
	"name": "a",
})
if ok {
	// wrong
}
```

<a name="any" />
### Any(source, predicate)

__Arguments__

* `source` - array or map
* `predicate` - func(element or value, index or key) bool

__Return__

* bool - any of the values that pass a truth test `predicate`

__Examples__

```go
arr := []int{ 1, 3 }
ok := Any(arr, func (n, _ int) bool {
	return n % 2 == 0
})
if ok {
	// wrong
}
```

<a name="anyBy" />
### AnyBy(source, properties)

__Arguments__

* `source` - array or map
* `properties` - map[string]interface{}

__Return__

* bool

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
	TestModel{ 3, "three" },
}
ok := AnyBy(arr, map[string]interface{}{
	"Id": 2,
	"name": "two",
})
if !ok {
	// wrong
}
```

<a name="asParallel" />
### Chain(source).AsParallel()...

__Support__
* `Each`

__Examples__

```go
arr := []int{ 1, 2, 3 }
Chain(arr).AsParallel().Each(func (n, i int) {
	// code
})
```

<a name="chain" />
### Chain(source)

__Arguments__

* `source` - array or map

__Return__

* interface{} - a wrapped object, wrapped objects until value is called

__Examples__

```go
res := Chain([]int{ 1, 2, 1, 4, 1, 3 }).Uniq(nil).Group(func (n, _ int) string {
	if (n % 2 == 0) {
		return "even"
	}

	return "odd"
}).Value()
dict := res.(map[string][]int)
if !(len(dict) == 2 && len(dict["odd]) == 2) {
	// wrong
}
```

<a name="clone" />
### Clone()

__Return__

* interface{}

__Examples__

```go
arr := []int{ 1, 2, 3 }
duplicate := Clone(arr)
ok := All(duplicate, func (n, i int) bool {
	return arr[i] == n
})
if !ok {
	// wrong
}
```

<a name="each" />
### Each(source, iterator)

__Arguments__

* `source` - array or map
* `iterator` - func(element or value, index or key)

__Examples__

```go
arr := []int{ 1, 2, 3 }
Each(arr, func (n, i int) {
	// code
})
```

<a name="find" />
### Find(source, predicate)

__Arguments__

* `source` - array or map
* `predicate` - func(element or value, index or key) bool

__Return__

* interface{}

__Examples__

```go
arr := []int{ 1, 2, 3, 4 }
n := Find(arr, func (n, _ int) bool {
	return n % 2 == 0
})
if n != 2 {
	// wrong
}
```

<a name="findBy" />
### FindBy(source, properties)

__Arguments__

* `source` - array or map
* `properties` - map[string]interface{}

__Return__

* interface{}

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
	TestModel{ 3, "three" },
}
res := FindBy(arr, map[string]interface{}{
	"id": 1,
})
if res == nil {
	// wrong
}

matcher := res.(TestModel)
if !(matcher.Id == arr[0].Id && matcher.Name == arr[0].Name) {
	// wrong
}
```

<a name="first" />
### First(source)

__Arguments__

* `source` - array or map

__Return__

* interface{}

__Examples__

```go
arr := []int{ 1, 2, 3 }
v := First(arr)
n, ok := v.(int)
if !(ok && n == 1) {
	//wrong
}

v = First(nil)
if v != nil {
	//wrong
}
```

<a name="group" />
### Group(source, keySelector)

__Arguments__

* `source` - array or map
* `keySelector` - func(element or value, index or key) anyType

__Return__

* interface{} - map[anyType][](element or value)

__Examples__

```go
v := Group([]int{ 1, 2, 3, 4, 5 }, func (n, _ int) string {
	if n % 2 == 0 {
		return "even"
	}
	return "odd"
})
dict, ok := v.(map[string][]int)
if !(ok && len(dict["even"]) == 2) {
	t.Error("wrong")
}
```

<a name="groupBy" />
### GroupBy(source, property)

__Arguments__

* `source` - array or map
* `property` - property name

__Return__

* interface{} - map[property type][](element or value)

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "a" },
	TestModel{ 2, "a" },
	TestModel{ 3, "b" },
	TestModel{ 4, "b" },
}
v := GroupBy(arr, "name")
dict, ok := v.(map[string][]TestModel)
if !(ok && len(dict) == 2) {
	t.Error("wrong")
}
```

<a name="index" />
### Index(source, indexSelector)

__Arguments__

* `source` - array or map
* `indexSelector` - func(element or value, index or key) anyType

__Return__

* interface{} - map[anyType](element or value)

__Examples__

```go
v, _ := Index([]string{ "a", "b" }, func (item string, _ int) string {
	return item
})
res, ok := v.(map[string]string)
if !(ok && res["a"] == "a") {
	// wrong
}
```

<a name="indexBy" />
### IndexBy(source, property)

__Arguments__

* `source` - array or map
* `property` - string

__Return__

* interface{} - map[propertyType](element or value)

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "a" },
	TestModel{ 2, "a" },
	TestModel{ 3, "b" },
	TestModel{ 4, "b" },
}
res := IndexBy(arr, "Name")
dict, ok := res.(map[string]TestModel)
if !(ok && len(dict) == 2) {
	// wrong
}
```

<a name="isArray" />
### IsArray(element)

__Arguments__

* `element` - object

__Return__

* bool

__Examples__

```go
if !IsArray([]int{}) {
	// wrong
}

if IsArray(map[string]int{}) {
	// wrong
}
```

<a name="isMatch" />
### IsMatch(element, properties)

__Arguments__

* `element` - object
* `properties` - map[string]interface{}

__Return__

* bool

__Examples__

```go
m := TestModel{ 1, "one" }
ok := IsMatch(nil, nil)
if ok {
	// wrong
}

ok = IsMatch(m, nil)
if ok {
	// wrong
}

ok = IsMatch(m, map[string]interface{}{
	"id": m.Id,
	"name": "a",
})
if ok {
	// wrong
}

ok = IsMatch(m, map[string]interface{}{
	"id": m.Id,
	"name": m.Name,
})
if !ok {
	// wrong
}
```

<a name="keys" />
### Keys()

__Arguments__

* `source` - map

__Return__

* interface{} - []keyType

__Examples__

```go
arr := []string{ "aa" }
v := Keys(arr)
if v != nil {
	// wrong
}

dict := map[int]string{	
	1: "a",
	2: "b",
	3: "c",
	4: "d",
}
v = Keys(dict)
res, ok := v.([]int)
if !(ok && len(res) == len(dict)) {
	// wrong
}
```

<a name="map" />
### Map(source, selector)

__Arguments__

* `source` - array or map
* `selector` - func(element, index or key) anyType

__Return__

* interface{} - an array of anyType

__Examples__

```go
arr := []string{ "11", "12", "13" }
v := Map(arr, func (s string, _ int) int {
	n, _ := strconv.Atoi(s)
	return n
})
res, ok := v.([]int)
if !(ok && len(res) == len(arr)) {
	// wrong
}
```

<a name="md5" />
### Md5(plaintext)

__Arguments__

* `plaintext` - string

__Return__

* string - md5 string

__Examples__

```go
if Md5("123456") != "e10adc3949ba59abbe56e057f20f883e" {
	// wrong
}	
```

<a name="parseJson" />
### ParseJson(str, container)

__Arguments__

* `str` - json string
* `container` - interface{}

__Return__

* error

__Examples__

```go
str := `["a","b"]`
var arr []string
err := ParseJson(str, &arr)
if !(err == nil && len(arr) == 2) {
	// wrong
}
```

<a name="mapBy" />
### MapBy(source, property)

__Arguments__

* `source` - array or map
* `property` - property name

__Return__

* interface{} - []propertyType

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "a" },
	TestModel{ 2, "a" },
	TestModel{ 3, "b" },
	TestModel{ 4, "b" },
}
v := MapBy(arr, "name")
res, ok := v.([]string)
if !(ok && len(res) == 4) {
	// wrong
}
```

<a name="pluck" />
### Pluck(source, property)

__Arguments__

* `source` - array
* `property` - string

__Return__

* interface{} - an array of property type

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
	TestModel{ 3, "three" },
}
v := Pluck(arr, "name")
res, ok := v.([]string)
if !(ok && len(res) == len(arr)) {
	// wrong
}

for i := 0; i < 3; i++ {
	if res[i] != arr[i].Name {
		// wrong
	}
}
```

<a name="property" />
### Property(name)

__Arguments__

* `name` - property name

__Return__

* func(interface{}) (interface{}, error)

__Examples__

```go
item := TestModel{ 1, "one" }

getAge := Property("age")
_, err := getAge(item)
if err == nil {
	// wrong
}

getName := Property("name")
name, err := getName(item)
if !(err == nil && name.(string) == item.Name) {
	// wrong
}
```

<a name="propertyRV" />
### Property(name)

__Arguments__

* `name` - property name

__Return__

* func(interface{}) (reflect.Value, error)

__Examples__

```go
item := TestModel{ 1, "one" }

getAgeRV := PropertyRV("age")
_, err := getAgeRV(item)
if err == nil {
	// wrong
}

getNameRV := PropertyRV("name")
nameRV, err := getNameRV(item)
if !(err == nil && nameRV.String() == item.Name) {
	// wrong
}
```

<a name="range" />
### Range(start, stop, step)

__Arguments__

* `start` - int
* `stop` - int
* `step` - int

__Return__

* []int

__Examples__

```go
arr := Range(0, 0, 1)
if len(arr) != 0 {
	// wrong
}

arr = Range(0, 10, 0)
if len(arr) != 0 {
	// wrong
}

arr = Range(10, 0, 1)
if len(arr) != 0 {
	// wrong
}

arr = Range(0, 2, 1)
if !(len(arr) == 2 && arr[0] == 0 && arr[1] == 1) {
	// wrong
}

arr = Range(0, 3, 2)
if !(len(arr) == 2 && arr[0] == 0 && arr[1] == 2) {
	// wrong
}
```

<a name="reduce" />
### Reduce(source, iterator)

__Arguments__

* `source` - array
* `iterator` - func(memo, element or value, key or index) memo
* `memo` - anyType

__Return__

* interface{} - memo

__Examples__

```go
v := Reduce([]int{ 1, 2 }, func (memo []int, n, _ int) []int {
	memo = append(memo, n)
	memo = append(memo, n + 10)
	return memo
}, make([]int, 0))
res, ok := v.([]int)
if !(ok && len(res) == 4) {
	// wrong
}

if !(res[0] == 1 && res[1] == 11 && res[2] == 2 && res[3] == 12) {
	// wrong
}
```

<a name="reject" />
### Reject(source, predicate)

__Arguments__

* `source` - array or map
* `predicate` - func(element or value, index or key) bool

__Return__

* interface{} - an array of all the values that without pass a truth test `predicate`

__Examples__

```go
arr := []int{ 1, 2, 3, 4 }
v := Reject(arr, func (n, i int) bool {
	return n % 2 == 0
})
res, ok := v.([]int)
if !(ok && len(res) == 2) {
	// wrong
}

if !(res[0] == 1 && res[1] == 3) {
	// wrong
}
```

<a name="rejectBy" />
### RejectBy(source, properties)

__Arguments__

* `source` - array or map
* `properties` - map[string]interface{}

__Return__

* interface{} - an array of all the values that without pass a truth test `properties`

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
	TestModel{ 3, "three" },
}
v := RejectBy(arr, map[string]interface{}{
	"Id": 1,
})
res, ok := v.([]TestModel)
if !(ok && len(res) == 2) {
	// wrong
}
```

<a name="select" />
### Select(source, predicate)

__Arguments__

* `source` - array or map
* `predicate` - func(element or value, index or key) bool

__Return__

* interface{} - an array of all the values that pass a truth test `predicate`

__Examples__

```go
arr := []int{ 1, 2, 3, 4 }
v := Select(arr, func (n, i int) bool {
	return n % 2 == 0
})
res, ok := v.([]int)
if !(ok && len(res) == 2) {
	t.Error("wrong length")
	return
}

if !(res[0] == 2 && res[1] == 4) {
	t.Error("wrong result")
}
```

<a name="selectBy" />
### SelectBy(source, properties)

__Arguments__

* `source` - array or map
* `properties` - map[string]interface{}

__Return__

* interface{} - an array of all the values that pass a truth test `properties`

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
	TestModel{ 3, "three" },
}
v := SelectBy(arr, map[string]interface{}{
	"Id": 1,
})
res, ok := v.([]TestModel)
if !(ok && len(res) == 1 && res[0] == arr[0]) {
	// wrong
}
```

<a name="size" />
### Size(source)

__Arguments__

* `source` - array or map

__Return__

* int

__Examples__

```go
dict := map[string]int{
	"a": 1,
	"b": 2,
	"c": 3,
}
if Size(dict) != len(dict) {
	// wrong
}
```

<a name="sort" />
### Sort(source, selector)

__Arguments__

* `source` - array or map
* `selector` - func(element, key or index) anyType

__Return__

* interface{} - an array of `source` that sorted

__Examples__

```go
arr := []int{ 1, 2, 3, 5 }
v := Sort([]int{ 5, 3, 2, 1 }, func (n, _ int) int {
	return n
})
res, ok := v.([]int)
if !(ok && len(res) == len(arr)) {
	// wrong
}

for i, n := range arr {
	if res[i] != n {
		// wrong
	}
}
```

<a name="sortBy" />
### SortBy(source, property)

__Arguments__

* `source` - array or map
* `property` - string

__Return__

* interface{}

__Examples__

```go
arr := []TestModel{
	TestModel{ 3, "three" },
	TestModel{ 1, "one" },
	TestModel{ 2, "two" },
}
v := SortBy(arr, "id")
res, ok := v.([]TestModel)
if !(ok && len(res) == len(arr)) {
	// wrong
}

if !(res[0].Id < res[1].Id && res[1].Id < res[2].Id) {
	// wrong
}
```

<a name="take" />
### Take(source, count)

__Arguments__

* `source` - array or map
* `count` - int

__Return__

* interface{}

__Examples__

```go
arr := []int{ 1, 2, 3 }
v := Take(arr, 1)
res, ok := v.([]int)
if !ok {
	// wrong
}

if res[0] != 1 {
	// wrong
}
```

<a name="toJson" />
### ToJson(value)

__Arguments__

* `value` - interface{}, anyType

__Return__

* string, error

__Examples__

```go
b := true
v, _ := ToJson(b)
if v != "true" {
	// wrong
}

str := "a"
v, _ = ToJson(str)
if v != str {
	// wrong
}

v, _ = ToJson(1)
if v != "1" {
	// wrong
}

arr := []int{ 1, 2, 3 }
v, _ = ToJson(arr)
if v != "[1,2,3]" {
	// wrong
}

obj := TestModel{ 1, "name" }
v, _ = ToJson(obj)
if v != `{"Id":1,"Name":"name"}` {
	// wrong
}
```

<a name="uniq" />
### Uniq(source, selector)

__Arguments__

* `source` - array
* `selector` - nil or func(element or value, index or key) anyType

__Return__

* interface{} - only the first occurence of each value is kept

__Examples__

```go
v := Uniq([]int{ 1, 2, 1, 4, 1, 3 }, func (n, _ int) int {
	return n % 2
})
res, ok := v.([]int)
if !(ok && len(res) == 2) {
	// wrong
}
```

<a name="uniqBy" />
### UniqBy(source, property)

__Arguments__

* `source` - array
* `property` - string

__Return__

* interface{}

__Examples__

```go
arr := []TestModel{
	TestModel{ 1, "one" },
	TestModel{ 2, "one" },
	TestModel{ 3, "one" },
}
v := UniqBy(arr, "Name")
res, ok := v.([]TestModel)
if !(ok && len(res) == 1) {
	// wrong
}
```

<a name="uuid" />
### UUID()

__Return__

* string - uuid string

__Examples__

```go
uuid := UUID()
//1a40272540e57d1c80e7b06042219d0c
```

<a name="values" />
### Values(source)

__Arguments__

* `source` - map

__Return__

* interface{} - an array of `source`'s values
* error

__Examples__

```go
dict := map[int]string{	
	1: "a",
	2: "b",
	3: "c",
	4: "d",
}
v, _ := Values(dict)
res, ok := v.([]string)
if !(ok && len(res) == len(dict)) {
	// wrong
}