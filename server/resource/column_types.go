package resource

import (
	"github.com/icrowley/fake"
	"github.com/satori/go.uuid"
	"time"
	"math/rand"
	"fmt"
	validator2 "gopkg.in/go-playground/validator.v9"
)

type Faker interface {
	Fake() string
}

type ColumnType struct {
	BlueprintType string
	Name          string
	Validations   []string
	Conformations []string
	ReclineType   string
}

func randate() time.Time {
	min := time.Date(1970, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Date(2070, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	delta := max - min

	sec := rand.Int63n(delta) + min
	return time.Unix(sec, 0)
}

func (ct ColumnType) Fake() interface{} {

	switch ct.Name {
	case "id":
		return uuid.NewV4().String()
	case "alias":
		return uuid.NewV4().String()
	case "date":
		return randate()
	case "time":
		return randate()
	case "day":
		return fake.Day()
	case "month":
		return fake.Month()
	case "year":
		return fake.Year(1990, 2018)
	case "minute":
		return rand.Intn(60)
	case "hour":
		return rand.Intn(24)
	case "datetime":
		return randate()
	case "email":
		return fake.EmailAddress()
	case "name":
		return fake.FullName()
	case "json":
		return "{}"
	case "password":
		return ""
	case "value":
		return rand.Intn(1000)
	case "truefalse":
		return rand.Intn(3) == 1
	case "timestamp":
		return randate().Unix()
	case "location.latitude":
		return fake.Latitude()
	case "location":
		return fmt.Sprintf("[%v, %v]", fake.Latitude(), fake.Longitude())
	case "location.longitude":
		return fake.Longitude()
	case "location.altitude":
		return rand.Intn(10000)
	case "color":
		return fake.HexColor()
	case "rating.10":
		return rand.Intn(11)
	case "measurement":
		return rand.Intn(5000)
	case "label":
		return fake.ProductName()
	case "content":
		return fake.Sentences()
	case "file":
		return ""
	case "url":
		return "https://places.com/"
	default:
		return ""
	}
}

/**
"string"
"number"
"integer"
"date"
"time"
"date-time"
"boolean"
"binary"
"geo_point"
 */

var ColumnTypes = []ColumnType{
	{
		Name:          "id",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{},
	},
	{
		Name:          "alias",
		BlueprintType: "string",
		ReclineType:   "string",
	},
	{
		Name:          "date",
		BlueprintType: "string",
		ReclineType:   "date",
	},
	{
		Name:          "time",
		BlueprintType: "string",
		ReclineType:   "time",
	},
	{
		Name:          "day",
		BlueprintType: "string",
		ReclineType:   "string",
	},
	{
		Name:          "month",
		BlueprintType: "number",
		ReclineType:   "string",
		Validations:   []string{"min=1,max=12"},
	},
	{
		Name:          "year",
		BlueprintType: "number",
		ReclineType:   "string",
		Validations:   []string{"min=1900,max=2100"},
	},
	{
		Name:          "minute",
		BlueprintType: "number",
		Validations:   []string{"min=0,max=59"},
	},
	{
		Name:          "hour",
		BlueprintType: "number",
		ReclineType:   "string",
	},
	{
		Name:          "datetime",
		BlueprintType: "string",
		ReclineType:   "date-time",
	},
	{
		Name:          "email",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"email"},
		Conformations: []string{"email"},
	},
	{
		Name:          "namespace",
		BlueprintType: "string",
		ReclineType:   "string",
	},
	{
		Name:          "name",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
		Conformations: []string{"name"},
	},
	{
		Name:          "encrypted",
		ReclineType:   "string",
		BlueprintType: "string",
	},
	{
		Name:          "json",
		ReclineType:   "string",
		BlueprintType: "string",
	},
	{
		Name:          "password",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
	},
	{
		Name:          "value",
		ReclineType:   "string",
		BlueprintType: "number",
	},
	{
		Name:          "truefalse",
		BlueprintType: "boolean",
		ReclineType:   "boolean",
	},
	{
		Name:          "timestamp",
		BlueprintType: "timestamp",
		ReclineType:   "date-time",
	},
	{
		Name:          "location",
		BlueprintType: "string",
		ReclineType:   "geo_point",
	},
	{
		Name:          "location.latitude",
		BlueprintType: "number",
		ReclineType:   "number",
		Validations:   []string{"latitude"},
	},
	{
		Name:          "location.longitude",
		BlueprintType: "number",
		ReclineType:   "number",
		Validations:   []string{"longitude"},
	},
	{
		Name:          "location.altitude",
		BlueprintType: "string",
		ReclineType:   "number",
	},
	{
		Name:          "color",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"iscolor"},
	},
	{
		Name:          "rating.10",
		BlueprintType: "number",
		ReclineType:   "string",
		Validations:   []string{"min=0,max=10"},
	},
	{
		Name:          "measurement",
		ReclineType:   "number",
		BlueprintType: "number",
	},
	{
		Name:          "label",
		ReclineType:   "string",
		BlueprintType: "string",
	},
	{
		Name:          "content",
		ReclineType:   "string",
		BlueprintType: "string",
	},
	{
		Name:          "file",
		BlueprintType: "string",
		ReclineType:   "binary",
		Validations:   []string{"base64"},
	},
	{
		Name:          "url",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"url"},
	},
	{
		Name:          "image",
		BlueprintType: "string",
		ReclineType:   "binary",
		Validations:   []string{"base64"},
	},
}

type ColumnTypeManager struct {
	ColumnMap map[string]ColumnType
}

var ColumnManager *ColumnTypeManager

func InitialiseColumnManager() {
	ColumnManager = &ColumnTypeManager{}
	ColumnManager.ColumnMap = make(map[string]ColumnType)
	for _, col := range ColumnTypes {
		ColumnManager.ColumnMap[col.Name] = col
	}
}

func (ctm *ColumnTypeManager) GetBlueprintType(colName string) string {
	return ctm.ColumnMap[colName].BlueprintType
}

func (ctm *ColumnTypeManager) GetFakedata(colTypeName string) string {
	return fmt.Sprintf("%v", ctm.ColumnMap[colTypeName].Fake())
}

func (ctm *ColumnTypeManager) IsValidValue(val string, colType string, validator *validator2.Validate) error {
	if ctm.ColumnMap[colType].Validations == nil || len(ctm.ColumnMap[colType].Validations) < 1 {
		return nil
	}
	return validator.Var(val, ctm.ColumnMap[colType].Validations[0])

}

var CollectionTypes = []string{
	"Pair",
	"Triplet",
	"Set",
	"OrderedSet",
}
