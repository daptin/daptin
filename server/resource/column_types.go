package resource

import (
  "github.com/icrowley/fake"
  "github.com/satori/go.uuid"
  "time"
  "math/rand"
  "fmt"
)

type Faker interface {
  Fake() string
}

type ColumnType struct {
  BlueprintType string
  Name          string
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

var ColumnTypes = []ColumnType{
  {
    Name:          "id",
    BlueprintType: "string",
  },
  {
    Name:          "alias",
    BlueprintType: "string",
  },
  {
    Name:          "date",
    BlueprintType: "string",
  },
  {
    Name:          "time",
    BlueprintType: "string",
  },
  {
    Name:          "day",
    BlueprintType: "string",
  },
  {
    Name:          "month",
    BlueprintType: "number",
  },
  {
    Name:          "year",
    BlueprintType: "number",
  },
  {
    Name:          "minute",
    BlueprintType: "number",
  },
  {
    Name:          "hour",
    BlueprintType: "number",
  },
  {
    Name:          "datetime",
    BlueprintType: "string",
  },
  {
    Name:          "email",
    BlueprintType: "string",
  },
  {
    Name:          "name",
    BlueprintType: "string",
  },
  {
    Name:          "encrypted",
    BlueprintType: "string",
  },
  {
    Name:          "json",
    BlueprintType: "string",
  },
  {
    Name:          "password",
    BlueprintType: "string",
  },
  {
    Name:          "value",
    BlueprintType: "number",
  },
  {
    Name:          "truefalse",
    BlueprintType: "boolean",
  },
  {
    Name:          "timestamp",
    BlueprintType: "timestamp",
  },
  {
    Name:          "location.latitude",
    BlueprintType: "string",
  },
  {
    Name:          "location.longitude",
    BlueprintType: "string",
  },
  {
    Name:          "location.altitude",
    BlueprintType: "string",
  },
  {
    Name:          "color",
    BlueprintType: "string",
  },
  {
    Name:          "rating.10",
    BlueprintType: "number",
  },
  {
    Name:          "measurement",
    BlueprintType: "number",
  },
  {
    Name:          "label",
    BlueprintType: "string",
  },
  {
    Name:          "content",
    BlueprintType: "string",
  },
  {
    Name:          "file",
    BlueprintType: "string",
  },
  {
    Name:          "url",
    BlueprintType: "string",
  },
  {
    Name:          "image",
    BlueprintType: "string",
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

var CollectionTypes = []string{
  "Pair",
  "Triplet",
  "Set",
  "OrderedSet",
}
