package fakerservice

import (
	"github.com/artpar/daptin/server/resource"
)

func NewFakeInstance(tableInfo resource.TableInfo) map[string]interface{} {

	newObject := make(map[string]interface{})

	for _, col := range tableInfo.Columns {
		if col.IsForeignKey {
			continue
		}

		if col.ColumnName == "id" {
			continue
		}

		fakeData := resource.ColumnManager.GetFakedata(col.ColumnType)

		newObject[col.ColumnName] = fakeData

	}

	return newObject

}
