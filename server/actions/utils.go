package actions

import (
	"github.com/artpar/conform"
	"github.com/artpar/xlsx/v2"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"strings"
)

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
