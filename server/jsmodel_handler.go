package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "strings"
  log "github.com/Sirupsen/logrus"
  "github.com/artpar/gocms/datastore"
)

var tableMap map[string]datastore.TableInfo

func CreateJsModelHandler(initConfig *CmsConfig) func(*gin.Context) {
  tableMap := make(map[string]datastore.TableInfo)
  for _, table := range initConfig.Tables {

    log.Infof("Default permission for [%v]: [%v]", table.TableName, table.Columns)

    tableMap[table.TableName] = table
  }

  return func(c *gin.Context) {
    typeName := strings.Split(c.Param("typename"), ".")[0]
    selectedTable, ok := tableMap[typeName]

    if !ok {
      c.AbortWithStatus(404)
      return
    }

    log.Infof("data: %v", selectedTable.Relations)

    cols := selectedTable.Columns
    actions := GetActionList(selectedTable.TableName, initConfig)

    res := map[string]interface{}{}

    for _, col := range cols {
      log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue)
      if col.ColumnName == "deleted_at" {
        continue
      }
      if col.ColumnName == "id" {
        continue
      }

      if col.IsForeignKey {
        continue
      }

      res[col.ColumnName] = col
    }

    for _, rel := range selectedTable.Relations {

      if rel.GetSubject() == selectedTable.TableName {
        r := "hasMany"
        if rel.GetRelation() == "belongs_to" {
          r = "hasOne"
        }
        res[rel.GetObjectName()] = NewJsonApiRelation(rel.GetObject(), r, "entity")
      } else {
        if (rel.GetRelation() == "belongs_to") {
          res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetObject(), "hasMany", "entity")
        } else {

        }
      }
    }

    jsModel := JsModel{
      ColumnModel: res,
      Actions: actions,
    }

    //res["__type"] = "string"
    c.JSON(200, jsModel)
    if true {
      return
    }

    //j, _ := json.Marshal(res)

    //c.String(200, "jsonApi.define('%v', %v)", typeName, string(j))

  }
}

func NewJsonApiRelation(name string, relationType string, columnType string) JsonApiRelation {

  return JsonApiRelation{
    Type: name,
    JsonApi: relationType,
    ColumnType: columnType,
  }

}

type JsonApiRelation struct {
  JsonApi    string `json:"jsonApi,omitempty"`
  ColumnType string `json:"columnType"`
  Type       string `json:"type,omitempty"`
}
