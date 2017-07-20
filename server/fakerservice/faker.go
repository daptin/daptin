package fakerservice

import (
  "github.com/artpar/goms/server/resource"
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

    if col.ColumnName == "deleted_at" {
      continue
    }

    fakeData := resource.ColumnManager.GetFakedata(col.ColumnType)

    newObject[col.ColumnName] = fakeData

  }

  return newObject

}
