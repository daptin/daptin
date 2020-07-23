package fieldtypes

import (
	"errors"
	"fmt"
	"github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

type EntityType int

func (t EntityType) String() string {
	switch t {
	case Time:
		return "time"
	case Date:
		return "date"
	case DateTime:
		return "datetime"
	case Ipaddress:
		return "ipaddress"
	case Money:
		return "money"
	case NumberFloat:
		return "number-float"
	case NumberInt:
		return "number-int"
	case None:
		return "none"
	case Boolean:
		return "boolean"
	case Latitude:
		return "location-latitude"
	case Longitude:
		return "location-longitude"
	case City:
		return "location-city"
	case Country:
		return "location-country"
	case Continent:
		return "location-continent"
	case State:
		return "location-state"
	case Pincode:
		return "location-pincode"
	case Timestamp:
		return "timestamp"
	case Rating5:
		return "rating5"
	case Rating10:
		return "rating10"
	case Rating100:
		return "rating100"
	case Content:
		return "content"
	case Label:
		return "label"
	case Alias:
		return "alias"
	case Color:
		return "color"
	case Json:
		return "json"
	case Email:
		return "email"
	case Name:
		return "name"
	case Id:
		return "id-col"
	}
	return "name-not-set"
}

func (t EntityType) MarshalJSON() ([]byte, error) {
	return []byte("\"" + t.String() + "\""), nil
}

const (
	DateTime EntityType = iota
	Id
	Time
	Date
	Ipaddress
	Money
	Rating5
	Rating10
	Rating100
	Timestamp
	NumberInt
	NumberFloat
	Boolean
	Latitude
	Longitude
	City
	Country
	Continent
	State
	Pincode
	Content
	Label
	Alias
	Color
	Json
	Email
	Namespace
	Name
	None
)

var (
	order = []EntityType{
		Id,
		Boolean,
		DateTime,
		Date,
		Time,
		Rating5,
		Rating10,
		Latitude,
		Longitude,
		Rating100,
		Timestamp,
		Ipaddress,
		City,
		Country,
		Continent,
		State,
		Pincode,
		NumberInt,
		NumberFloat,
		Content,
		Label,
		Alias,
		Color,
		Json,
		Email,
		Namespace,
		Name,
		Money,
	}
)

type DataTypeDetector struct {
	DataType         EntityType
	DetectorType     string
	DetectorFunction func(string) (bool, interface{})
	Attributes       map[string]interface{}
}

func IsNumber(d string) (bool, interface{}) {
	d = strings.ToLower(d)
	in := sort.SearchStrings(unknownNumbers, d)
	if in < len(unknownNumbers) && unknownNumbers[in] == d {
		log.Infof("One of the unknowns - %v : %d", d, sort.SearchStrings(unknownNumbers, strings.ToLower(d)))
		return true, 0
	}
	v, err := strconv.ParseFloat(d, 64)
	if err == nil {
		return true, v
	}
	//log.Infof("Parse %v as float failed - %v", d, err)
	v1, err := strconv.ParseInt(d, 10, 64)
	if err == nil {
		return true, v1
	}
	//log.Infof("Parse %v as int failed - %v", d, err)
	return false, 0
}

func IsFloat(d string) (bool, interface{}) {
	d = strings.ToLower(d)
	in := sort.SearchStrings(unknownNumbers, d)
	if in < len(unknownNumbers) && unknownNumbers[in] == d {
		log.Infof("One of the unknowns - %v : %d", d, sort.SearchStrings(unknownNumbers, strings.ToLower(d)))
		return true, 0
	}
	v, err := strconv.ParseFloat(d, 64)
	if err == nil {
		return true, v
	}
	//log.Infof("Parse %v as int failed - %v", d, err)
	return false, 0
}

func IsInt(d string) (bool, interface{}) {

	if d == "-" {
		return true, 0
	}

	d = strings.ToLower(d)

	if d == "na" {
		return true, 0
	}

	in := sort.SearchStrings(unknownNumbers, d)
	if in < len(unknownNumbers) && unknownNumbers[in] == d {
		log.Infof("One of the unknowns - %v : %d", d, sort.SearchStrings(unknownNumbers, strings.ToLower(d)))
		return true, 0
	}

	//log.Infof("Parse %v as float failed - %v", d, err)
	v1, err := strconv.ParseInt(d, 10, 64)
	if err == nil {
		return true, v1
	}
	//log.Infof("Parse %v as int failed - %v", d, err)
	return false, 0
}

var detectorMap = map[EntityType]DataTypeDetector{
	Name: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "[a-zA-Z]+ [a-zA-Z]+",
		},
	},
	Namespace: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "[a-zA-Z0-9]([\\\\\\/\\.])([a-zA-Z0-9]+[\\\\\\/\\.]?)",
		},
	},
	Email: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "[a-zA-Z0-9_]+@[0-9a-zA-Z_-]+\\.[a-z]{2,10}(\\.[a-z]{2,10})?",
		},
	},
	Json: {
		DetectorType: "function",
		DetectorFunction: func(s string) (bool, interface{}) {
			var variab interface{}
			err := json.Unmarshal([]byte(s), &variab)
			if err != nil {
				return false, nil
			}
			return true, variab
		},
	},
	Color: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "#[0-9a-f]{3,6}",
		},
	},
	Alias: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "[a-f0-9]{8}\\-[a-f0-9]{4}\\-4[a-f0-9]{3}\\-(8|9|a|b)[a-f0-9]{3‌​}\\-[a-f0-9]{12}",
		},
	},
	Label: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "(.{3,100})",
		},
	},
	Content: {
		DetectorType: "regex",
		Attributes: map[string]interface{}{
			"regex": "(.{15,100})",
		},
	},
	Time: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			//fmt.Printf("Try to parse [%v] with mtime\n", d)
			t, _, err := GetTime(d)
			//fmt.Errorf("Fail to parse [%v] with mtime: %v\n", d, err)
			if err == nil {
				return true, t
			}
			return false, time.Now()
		},
	},
	Timestamp: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			//fmt.Printf("Try to parse [%v] with mtime\n", d)

			i, err := strconv.ParseInt(d, 10, 64)
			if err != nil {
				return false, d
			}

			if i < 100000000 {
				return false, d
			}

			tm := time.Unix(i, 0)

			return true, tm
		},
	},
	Date: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			t, _, err := GetDate(d)
			if err == nil {
				return true, t
			}
			return false, time.Now()
		},
	},
	DateTime: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			t, _, err := GetDateTime(d)
			if err == nil {
				return true, t
			}
			return false, time.Now()
		},
	},
	Ipaddress: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			s := net.ParseIP(d)
			if s != nil {
				return true, net.IP("")
			}
			return false, s
		},
	},
	Money: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			r := regexp.MustCompile("^([a-zA-Z]{0,3}\\.? )?[0-9]+\\.[0-9]{0,2}([a-zA-Z]{0,3})?")
			return r.MatchString(d), d
		},
	},
	Boolean: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			d = strings.ToLower(d)
			switch d {
			case "yes":
			case "true":
			case "1":
				d = "true"
			case "no":
			case "0":
			case "false":
				d = "false"
			}
			r, err := strconv.ParseBool(d)
			if err != nil {
				return false, false
			}
			return true, r
		},
	},
	Rating5: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			numberOk, nValue := IsInt(d)

			if !numberOk {
				return false, d
			}

			nInt, ok := nValue.(int)
			if ok {
				if nInt <= 5 {
					return true, nInt
				} else {
					return false, nInt
				}
			}

			nFloat, ok := nValue.(float64)
			if ok {
				if nFloat <= 5.0 {
					return true, nFloat
				} else {
					return false, nFloat
				}
			}
			return false, nValue

		},
	},
	Rating10: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {

			numberOk, nValue := IsInt(d)

			if !numberOk {
				return false, d
			}

			nInt, ok := nValue.(int)
			if ok {
				if nInt <= 10 {
					return true, nInt
				} else {
					return false, nInt
				}
			}

			nFloat, ok := nValue.(float64)
			if ok {
				if nFloat <= 10.0 {
					return true, nFloat
				} else {
					return false, nFloat
				}
			}
			return false, nValue

		},
	},
	Rating100: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {

			numberOk, nValue := IsInt(d)

			if !numberOk {
				return false, d
			}

			nInt, ok := nValue.(int)
			if ok {
				if nInt <= 100 {
					return true, nInt
				} else {
					return false, nInt
				}
			}

			nFloat, ok := nValue.(float64)
			if ok {
				if nFloat <= 100.0 {
					return true, nFloat
				} else {
					return false, nFloat
				}
			}
			return false, nValue

		},
	},
	NumberInt: {
		DetectorType:     "function",
		DetectorFunction: IsInt,
	},
	NumberFloat: {
		DetectorType:     "function",
		DetectorFunction: IsFloat,
	},
	Latitude: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {

			var realFloatValue float64
			isFloat, floatValue := IsFloat(d)
			isInt, _ := IsInt(d)
			if isInt {
				return false, nil
			}

			intVal, isInt := floatValue.(int)

			if !isInt {
				floatVal, isReallyFloat := floatValue.(float64)

				if !isReallyFloat {
					return false, floatValue
				}
				realFloatValue = floatVal
			} else {
				realFloatValue = float64(intVal)
			}

			if !isFloat || realFloatValue > 180.0 {
				return false, floatValue
			}

			return true, floatValue

		},
	},
	Longitude: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {

			var realFloatValue float64
			isFloat, floatValue := IsFloat(d)
			isInt, _ := IsInt(d)
			if isInt {
				return false, nil
			}
			intVal, isInt := floatValue.(int)

			if !isInt {
				floatVal, isReallyFloat := floatValue.(float64)

				if !isReallyFloat {
					return false, floatValue
				}
				realFloatValue = floatVal
			} else {
				realFloatValue = float64(intVal)
			}

			if !isFloat || realFloatValue > 90.0 {
				return false, floatValue
			}

			return true, floatValue

		},
	},
	None: {
		DetectorType: "function",
		DetectorFunction: func(d string) (bool, interface{}) {
			return true, d
		},
	},
}

var (
	unknownNumbers = sort.StringSlice([]string{"na", "n/a", "-"})
)

func ConvertValues(d []string, typ EntityType) ([]interface{}, error) {
	converted := make([]interface{}, len(d))
	converter, ok := detectorMap[typ]
	if !ok {
		log.Infof("Converter not found for %v", typ)
		return converted, errors.New("Converter not found for " + typ.String())
	}
	for i, v := range d {
		ok, val := converter.DetectorFunction(v)
		if !ok {
			// log.Infof("Conversion of %s as %v failed", v, typ)
			continue
		}
		converted[i] = val
	}
	return converted, nil
}

func checkStringsAgainstDetector(d []string, detect DataTypeDetector) (ok bool, unidentified []string) {
	unidentified = make([]string, 0)

	ok = true
	var detectorFunction func(string) (bool, interface{})

	switch detect.DetectorType {
	case "function":
		detectorFunction = detect.DetectorFunction
	case "regex":
		reg := detect.Attributes["regex"].(string)

		detectorFunction = (func(reg string) func(string) (bool, interface{}) {
			compiled, err := regexp.Compile(reg)
			if err != nil {
				log.Errorf("Failed to compile string as regex: %v", err)
				return func(s string) (bool, interface{}) {
					return false, nil
				}
			}
			return func(s string) (bool, interface{}) {
				thisOk := compiled.MatchString(s)
				return thisOk, s
			}
		})(reg)

	case "regex-list":
		reg := detect.Attributes["regex"].([]string)

		detectorFunction = (func(reg []string) func(string) (bool, interface{}) {
			compiledRegexs := make([]*regexp.Regexp, 0)

			for _, r := range reg {
				c, e := regexp.Compile(r)
				log.Errorf("Failed to compile string as regex: %v", e)
				//return func(s string) (bool, interface{}) {
				//	return false, nil
				//}
				compiledRegexs = append(compiledRegexs, c)
			}

			return func(s string) (bool, interface{}) {

				for _, compiled := range compiledRegexs {
					thisOk := compiled.MatchString(s)
					return thisOk, s
				}
				return false, nil
			}
		})(reg)

	}

	for _, s := range d {
		t := strings.TrimSpace(s)
		thisOk, _ := detectorFunction(t)
		if !thisOk {
			unidentified = append(unidentified, s)
			ok = false
			break
		}
	}

	if ok {
		return ok, unidentified
	}

	return false, unidentified
}

func DetectType(d []string) (entityType EntityType, hasHeaders bool, err error) {
	hasHeaders = false
	var unidentified []string
	for _, typeInfo := range order {
		detect, ok := detectorMap[typeInfo]
		if !ok {
			//log.Infof("No detectorMap for type [%v]", typeInfo)
			continue
		}

		//log.Infof("Detector for type [%v]", typeInfo)
		ok, unidentified = checkStringsAgainstDetector(d, detect)

		if ok {
			return typeInfo, false, nil
		} else {
			//log.Infof("Column was not identified: %v", typeInfo)
		}
	}

	foundType := None
	columnHeader := d[0]
	typeByColumnName := columnTypeFromName(columnHeader)
	if typeByColumnName != None {
		foundType = typeByColumnName
	}

	if foundType == None {
		hasHeaders = true
		for _, typeInfo := range order {
			detect, ok := detectorMap[typeInfo]
			if !ok {
				//log.Infof("No detectorMap for type [%v]", typeInfo)
				continue
			}

			//log.Infof("Detector for type [%v]", typeInfo)
			ok, unidentified = checkStringsAgainstDetector(d[1:], detect)
			if ok {
				return typeInfo, hasHeaders, nil
			} else {
				//log.Infof("Column was not identified: %v", typeInfo)
			}
		}
	}

	if foundType != None {
		return foundType, hasHeaders, nil
	}

	return None, hasHeaders, errors.New(fmt.Sprintf("Failed to identify - %v", unidentified))
}

var nameMap = map[EntityType][]string{
	Id:        {"id"},
	Money:     {"price", "income", "amount", "wage", "cost", "sale", "profit", "asset", "marketvalue"},
	Latitude:  {"lat", "latitude"},
	Longitude: {"lon", "long", "longitude"},
	City:      {"city"},
	Country:   {"country"},
	State:     {"state"},
	Continent: {"continent"},
	Pincode:   {"pincode", "zipcode"},
}

func columnTypeFromName(name string) EntityType {
	name = strings.ToLower(name)
	for typ, names := range nameMap {
		for _, n := range names {
			if strings.HasSuffix(name, n) {
				log.Infof("Selecting type %s because of Suffix %s in %s", typ.String(), n, name)
				return typ
			}
			if strings.HasPrefix(name, n) {
				log.Infof("Selecting type %s because of Prefix %s in %s", typ.String(), n, name)
				return typ
			}

			if len(n) > 5 && strings.Index(name, n) > -1 {
				log.Infof("Selecting type %s because of Prefix %s in %s", typ.String(), n, name)
				return typ
			}
		}
	}
	return None
}
