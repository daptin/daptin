package server

import (
  "gopkg.in/gin-gonic/gin.v1"
  "strings"
  log "github.com/Sirupsen/logrus"
  "github.com/artpar/gocms/datastore"
  "errors"
  "github.com/artpar/api2go"
)

func CreateJsModelHandler(initConfig *CmsConfig) func(*gin.Context) {

  return func(c *gin.Context) {
    typeName := strings.Split(c.Param("typename"), ".")[0]
    //resource := resources[typeName]
    var selectedTable *datastore.TableInfo

    for _, t := range initConfig.Tables {
      if t.TableName == typeName {
        selectedTable = &t
        break
      }
    }

    if selectedTable == nil {
      c.AbortWithStatus(404)
      return
    }

    log.Infof("data: %v", selectedTable.Relations)

    if selectedTable == nil {
      c.AbortWithError(404, errors.New("Invalid type"))
      return
    }
    cols := selectedTable.Columns
    actions := GetActionList(selectedTable.TableName, initConfig)

    res := map[string]interface{}{}

    for _, col := range cols {
      if col.ColumnName == "deleted_at" {
        continue
      }
      if col.ColumnName == "id" {
        continue
      }

      _, ok := api2go.EndsWith(col.ColumnName, "_id")
      if ok && col.ColumnName != "reference_id" {
        log.Infof("Column [%v] is relation ", col.ColumnName)
        //res[typeOfOtherEntity] = NewJsonApiRelation(typeOfOtherEntity, "hasOne", "entity")
      } else {
        //res[col.ColumnName] = NewJsonApiRelation("", "", col.ColumnType)
        res[col.ColumnName] = col
      }

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

