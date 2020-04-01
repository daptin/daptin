package resource

import (
	"crypto/md5"
	"fmt"
	"github.com/artpar/go.uuid"
	"github.com/graphql-go/graphql"
	"github.com/icrowley/fake"
	log "github.com/sirupsen/logrus"
	validator2 "gopkg.in/go-playground/validator.v9"
	"math/rand"
	"strings"

	"time"
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
	DataTypes     []string
	GraphqlType   graphql.Type
}

func randate() time.Time {
	min := time.Date(1980, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	max := time.Date(2020, 1, 0, 0, 0, 0, 0, time.UTC).Unix()
	delta := max - min

	sec := rand.Int63n(delta) + min
	return time.Unix(sec, 0)
}

func (ct ColumnType) Fake() interface{} {

	switch ct.Name {
	case "id":
		u, _ := uuid.NewV4()
		return u.String()
	case "alias":
		u, _ := uuid.NewV4()
		return u.String()
	case "date":
		return randate().Format("2006-01-02")
	case "time":
		return randate().Format("15:04:05")
	case "day":
		return fake.Day()
	case "enum":
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
		return randate().Format(time.RFC3339)
	case "email":
		return fake.EmailAddress()
	case "name":
		return fake.FullName()
	case "json":
		return "{}"
	case "password":
		pass, _ := BcryptHashString(fake.SimplePassword())
		return pass
	case "bcrypt":
		pass, _ := BcryptHashString(fake.SimplePassword())
		return pass
	case "md5-bcrypt":
		pass, _ := BcryptHashString(fake.SimplePassword())
		digest := md5.New()
		digest.Write([]byte(pass))
		hash := digest.Sum(nil)
		return fmt.Sprintf("%x", hash)
	case "md5":
		digest := md5.New()
		digest.Write([]byte(fake.SimplePassword()))
		hash := digest.Sum(nil)
		return fmt.Sprintf("%x", hash)
	case "value":
		return rand.Intn(1000)
	case "truefalse":
		return rand.Intn(2)
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
	case "image":
		return ""
	case "video":
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
		DataTypes:     []string{"varchar(20)", "varchar(10)"},
		GraphqlType:   graphql.ID,
	},
	{
		Name:          "alias",
		BlueprintType: "string",
		ReclineType:   "string",
		DataTypes:     []string{"varchar(100)", "varchar(20)", "varchar(10)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "enum",
		BlueprintType: "string",
		ReclineType:   "string",
		DataTypes:     []string{"varchar(50)", "varchar(20)", "varchar(10)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "date",
		BlueprintType: "string",
		ReclineType:   "date",
		DataTypes:     []string{"timestamp"},
		GraphqlType:   graphql.DateTime,
	},
	{
		Name:          "time",
		BlueprintType: "string",
		ReclineType:   "time",
		DataTypes:     []string{"timestamp"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "day",
		BlueprintType: "string",
		ReclineType:   "string",
		DataTypes:     []string{"varchar(10)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "month",
		BlueprintType: "number",
		ReclineType:   "string",
		Validations:   []string{"min=1,max=12"},
		DataTypes:     []string{"int(4)"},
		GraphqlType:   graphql.Int,
	},
	{
		Name:          "year",
		BlueprintType: "number",
		ReclineType:   "string",
		Validations:   []string{"min=100,max=2100"},
		DataTypes:     []string{"int(4)"},
		GraphqlType:   graphql.Int,
	},
	{
		Name:          "minute",
		BlueprintType: "number",
		Validations:   []string{"min=0,max=59"},
		DataTypes:     []string{"int(4)"},
		GraphqlType:   graphql.Int,
	},
	{
		Name:          "hour",
		BlueprintType: "number",
		ReclineType:   "string",
		DataTypes:     []string{"int(4)"},
		GraphqlType:   graphql.Int,
	},
	{
		Name:          "datetime",
		BlueprintType: "string",
		ReclineType:   "date-time",
		DataTypes:     []string{"timestamp"},
		GraphqlType:   graphql.DateTime,
	},
	{
		Name:          "email",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"email"},
		Conformations: []string{"email"},
		DataTypes:     []string{"varchar(100)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "namespace",
		BlueprintType: "string",
		ReclineType:   "string",
		DataTypes:     []string{"varchar(200)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "name",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
		Conformations: []string{"name"},
		DataTypes:     []string{"varchar(100)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "encrypted",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"varchar(100)", "varchar(500)", "varchar(500)", "text"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "json",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"text", "varchar(100)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "password",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
		DataTypes:     []string{"varchar(200)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "md5",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
		DataTypes:     []string{"varchar(200)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "bcrypt",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
		DataTypes:     []string{"varchar(200)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "md5-bcrypt",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"required"},
		DataTypes:     []string{"varchar(200)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "value",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"varchar(100)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "truefalse",
		BlueprintType: "boolean",
		ReclineType:   "boolean",
		DataTypes:     []string{"boolean"},
		GraphqlType:   graphql.Boolean,
	},
	{
		Name:          "timestamp",
		BlueprintType: "string",
		ReclineType:   "date-time",
		DataTypes:     []string{"timestamp"},
		GraphqlType:   graphql.DateTime,
	},
	{
		Name:          "location",
		BlueprintType: "string",
		ReclineType:   "geo_point",
		DataTypes:     []string{"varchar(50)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "location.latitude",
		BlueprintType: "number",
		ReclineType:   "number",
		Validations:   []string{"latitude"},
		DataTypes:     []string{"float(7,4)"},
		GraphqlType:   graphql.Float,
	},
	{
		Name:          "location.longitude",
		BlueprintType: "number",
		ReclineType:   "number",
		Validations:   []string{"longitude"},
		DataTypes:     []string{"float(7,4)"},
		GraphqlType:   graphql.Float,
	},
	{
		Name:          "location.altitude",
		BlueprintType: "number",
		ReclineType:   "number",
		DataTypes:     []string{"float(7,4)"},
		GraphqlType:   graphql.Float,
	},
	{
		Name:          "color",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"iscolor"},
		DataTypes:     []string{"varchar(50)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "rating",
		BlueprintType: "number",
		ReclineType:   "number",
		Validations:   []string{"min=0,max=10"},
		DataTypes:     []string{"int(4)"},
		GraphqlType:   graphql.Int,
	},
	{
		Name:          "measurement",
		ReclineType:   "number",
		BlueprintType: "number",
		DataTypes:     []string{"int(10)"},
		GraphqlType:   graphql.Int,
	},
	{
		Name:          "float",
		ReclineType:   "number",
		BlueprintType: "number",
		DataTypes:     []string{"float(7,4)"},
		GraphqlType:   graphql.Float,
	},
	{
		Name:          "label",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"varchar(100)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "hidden",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"varchar(100)"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "content",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"text"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "html",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"text"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "markdown",
		ReclineType:   "string",
		BlueprintType: "string",
		DataTypes:     []string{"text"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "file",
		BlueprintType: "string",
		ReclineType:   "binary",
		Validations:   []string{"base64"},
		DataTypes:     []string{"blob"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "json",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"text"},
		DataTypes:     []string{"JSON"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "image",
		BlueprintType: "string",
		ReclineType:   "binary",
		Validations:   []string{"base64"},
		DataTypes:     []string{"blob"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "gzip",
		BlueprintType: "string",
		ReclineType:   "binary",
		Validations:   []string{"base64"},
		DataTypes:     []string{"blob"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "video",
		BlueprintType: "string",
		ReclineType:   "binary",
		Validations:   []string{"base64"},
		DataTypes:     []string{"blob"},
		GraphqlType:   graphql.String,
	},
	{
		Name:          "url",
		BlueprintType: "string",
		ReclineType:   "string",
		Validations:   []string{"url"},
		DataTypes:     []string{"varchar(500)"},
		GraphqlType:   graphql.String,
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

func (ctm *ColumnTypeManager) GetBlueprintType(columnType string) string {
	return ctm.ColumnMap[columnType].BlueprintType
}
func (ctm *ColumnTypeManager) GetGraphqlType(columnType string) graphql.Type {
	col := strings.Split(columnType, ".")[0]
	if _, ok := ctm.ColumnMap[col]; !ok {
		log.Printf("No column definition for type: %v", columnType)
		return graphql.String
	}
	return ctm.ColumnMap[col].GraphqlType
}

func (ctm *ColumnTypeManager) GetFakeData(colTypeName string) string {
	return fmt.Sprintf("%v", ctm.ColumnMap[colTypeName].Fake())
}

func (ctm *ColumnTypeManager) IsValidValue(val string, colType string, validator *validator2.Validate) error {
	if ctm.ColumnMap[colType].Validations == nil || len(ctm.ColumnMap[colType].Validations) < 1 {
		return nil
	}
	return validator.Var(val, ctm.ColumnMap[colType].Validations[0])

}
