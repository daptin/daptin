package resource

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/artpar/conform"
	"github.com/artpar/xlsx/v2"
	jsoniter "github.com/json-iterator/go"
	log "github.com/sirupsen/logrus"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

func StringOrEmpty(value interface{}) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case []byte:
		return string(typedValue)
	default:
		return ""
	}
}

func resourceRowBool(value interface{}) (bool, error) {
	switch typedValue := value.(type) {
	case bool:
		return typedValue, nil
	case int:
		return typedValue == 1, nil
	case int64:
		return typedValue == 1, nil
	case int32:
		return typedValue == 1, nil
	case float64:
		return typedValue == 1, nil
	case string:
		if strings.EqualFold(typedValue, "true") {
			return true, nil
		}
		if strings.EqualFold(typedValue, "false") || typedValue == "" {
			return false, nil
		}
		parsed, err := strconv.ParseInt(typedValue, 10, 32)
		if err != nil {
			return false, err
		}
		return parsed == 1, nil
	case nil:
		return false, nil
	default:
		return false, fmt.Errorf("unsupported boolean resource value type [%T]", value)
	}
}

func ResourceRowInt64(value interface{}) (int64, error) {
	switch typedValue := value.(type) {
	case int64:
		return typedValue, nil
	case int:
		return int64(typedValue), nil
	case int32:
		return int64(typedValue), nil
	case uint64:
		if typedValue > math.MaxInt64 {
			return 0, fmt.Errorf("uint64 value exceeds int64")
		}
		return int64(typedValue), nil
	case []uint8:
		return strconv.ParseInt(string(typedValue), 10, 64)
	case string:
		return strconv.ParseInt(typedValue, 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert [%T] to int64", value)
	}
}

func EndsWith(str string, endsWith string) (string, bool) {
	if len(endsWith) > len(str) {
		return "", false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return "", false
	}

	suffix := str[len(str)-len(endsWith):]
	prefix := str[:len(str)-len(endsWith)]
	i := suffix == endsWith
	return prefix, i

}

func EndsWithCheck(str string, endsWith string) bool {
	if len(endsWith) > len(str) {
		return false
	}

	if len(endsWith) == len(str) && endsWith != str {
		return false
	}

	suffix := str[len(str)-len(endsWith):]
	i := suffix == endsWith
	return i

}

func BeginsWithCheck(str string, beginsWith string) bool {
	if len(beginsWith) > len(str) {
		return false
	}

	if len(beginsWith) == len(str) && beginsWith != str {
		return false
	}

	prefix := str[:len(beginsWith)]
	i := prefix == beginsWith
	//log.Printf("Check [%v] begins with [%v]: %v", str, beginsWith, i)
	return i

}

func SmallSnakeCaseText(str string) string {
	transformed := conform.TransformString(str, "lower,snake")
	return transformed
}

func GetDataArray(sheet *xlsx.Sheet) (dataMap []map[string]interface{}, columnNames []string, err error) {

	data := make([]map[string]interface{}, 0)

	rowCount := sheet.MaxRow
	columnCount := sheet.MaxCol

	log.Printf("Sheet has %d rows", rowCount)
	log.Printf("Sheet has %d cols", columnCount)

	if columnCount < 1 {
		err = errors.New("Sheet has 0 columns")
		return
	}

	if rowCount < 2 {
		err = errors.New("Sheet has less than 2 rows")
		return
	}

	//columnNames = make([]string, 0)
	properColumnNames := make([]string, 0)

	headerRow, _ := sheet.Row(0)

	for i := 0; i < columnCount; i++ {
		colName := headerRow.GetCell(i).Value
		if len(colName) < 1 {
			//err = errors.New(fmt.Sprintf("Column %d name has less then 3 characters", i+1))
			break
		}
		//columnNames = append(columnNames, colName)
		properColumnNames = append(properColumnNames, SmallSnakeCaseText(colName))
	}

	for i := 1; i < rowCount; i++ {
		emptyRow := true

		dataMap := make(map[string]interface{})

		currentRow, _ := sheet.Row(i)
		cCount := columnCount
		for j := 0; j < cCount; j++ {
			i2 := currentRow.GetCell(j).Value
			if strings.TrimSpace(i2) == "" {
				continue
			}
			emptyRow = false
			dataMap[properColumnNames[j]] = i2
		}
		if !emptyRow {
			data = append(data, dataMap)
		}
	}

	return data, properColumnNames, nil

}
