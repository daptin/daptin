package fakerservice

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
)

func NewFakeInstance(columns []api2go.ColumnInfo) map[string]interface{} {

	newObject := make(map[string]interface{})

	for _, col := range columns {
		if col.IsForeignKey {
			continue
		}

		if col.ColumnName == "id" {
			continue
		}

		fakeData := resource.ColumnManager.GetFakeData(col.ColumnType)

		newObject[col.ColumnName] = fakeData

	}

	return newObject

}
