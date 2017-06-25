package server

import (
	"github.com/artpar/api2go"
	"github.com/artpar/goms/server/resource"
	"github.com/gorilla/context"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	"gopkg.in/gin-gonic/gin.v1"
	"net/http"
	"strings"
)

func CreateJsModelHandler(initConfig *resource.CmsConfig) func(*gin.Context) {
	tableMap := make(map[string]resource.TableInfo)
	for _, table := range initConfig.Tables {

		//log.Infof("Default permission for [%v]: [%v]", table.TableName, table.Columns)

		tableMap[table.TableName] = table
	}

	worlds, _, err := cruds["world"].GetRowsByWhereClause("world", squirrel.Eq{"deleted_at": nil})
	if err != nil {
		log.Errorf("Failed to get worlds list")
	}

	worldToReferenceId := make(map[string]string)

	for _, world := range worlds {
		worldToReferenceId[world["table_name"].(string)] = world["reference_id"].(string)
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

		//actions := GetActionList(selectedTable.TableName, initConfig)
		actions, err := cruds["world"].GetActionsByType(selectedTable.TableName)

		if err != nil {
			log.Errorf("Failed to get actions by type: %v", err)
		}

		pr := &http.Request{
			Method: "GET",
		}

		params := make(map[string][]string)
		req := api2go.Request{
			PlainRequest: pr,
			QueryParams:  params,
		}

		worldRefId := worldToReferenceId[typeName]

		params["worldName"] = []string{"smd_id"}
		params["world_id"] = []string{worldRefId}

		context.Set(pr, "user_id", context.Get(c.Request, "user_id"))
		context.Set(pr, "usergroup_id", context.Get(c.Request, "usergroup_id"))

		smdList := make([]map[string]interface{}, 0)

		_, result, err := cruds["smd"].PaginatedFindAll(req)

		if err != nil {
			log.Infof("Failed to get world SMD: %v", err)
		} else {
			models := result.Result().([]*api2go.Api2GoModel)
			for _, m := range models {
				if m.GetAttributes()["__type"].(string) == "smd" {
					smdList = append(smdList, m.GetAttributes())
				}
			}

		}

		res := map[string]interface{}{}

		for _, col := range cols {
			log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue)

			if col.ExcludeFromApi {
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
				if rel.GetRelation() == "belongs_to" || rel.GetRelation() == "has_one" {
					r = "hasOne"
				}
				res[rel.GetObjectName()] = NewJsonApiRelation(rel.GetObject(), r, "entity")
			} else {
				if rel.GetRelation() == "belongs_to" {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), "hasMany", "entity")
				} else if rel.GetRelation() == "has_one" {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), "hasOne", "entity")
				} else {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), "hasMany", "entity")
				}
			}
		}
		res["__type"] = api2go.ColumnInfo{
			Name:       "type",
			ColumnName: "__type",
			ColumnType: "hidden",
		}

		jsModel := JsModel{
			ColumnModel:   res,
			Actions:       actions,
			StateMachines: smdList,
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
		Type:       name,
		JsonApi:    relationType,
		ColumnType: columnType,
	}

}

type JsonApiRelation struct {
	JsonApi    string `json:"jsonApi,omitempty"`
	ColumnType string `json:"columnType"`
	Type       string `json:"type,omitempty"`
}
