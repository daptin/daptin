package server

import (
	"github.com/artpar/api2go"
	"github.com/artpar/goms/server/resource"
	log "github.com/sirupsen/logrus"
	"gopkg.in/Masterminds/squirrel.v1"
	"gopkg.in/gin-gonic/gin.v1"
	"net/http"
	"strings"
	"github.com/artpar/goms/server/apiblueprint"
)

func CreateApiBlueprintHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(ctx *gin.Context) {
	return func(c *gin.Context) {
		c.String(200, "%s", apiblueprint.BuildApiBlueprint(initConfig, cruds))
	}
}

func CreateJsModelHandler(initConfig *resource.CmsConfig) func(*gin.Context) {
	tableMap := make(map[string]resource.TableInfo)
	for _, table := range initConfig.Tables {

		//log.Infof("Default permission for [%v]: [%v]", table.TableName, table.Columns)

		tableMap[table.TableName] = table
	}

	streamMap := make(map[string]resource.StreamContract)
	for _, stream := range initConfig.Streams {
		streamMap[stream.StreamName] = stream
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
		selectedTable, isTable := tableMap[typeName]

		if !isTable {

			selectedStream, isStream := streamMap[typeName]

			if !isStream {
				c.AbortWithStatus(404)
				return

			} else {
				selectedTable = resource.TableInfo{}
				selectedTable.TableName = selectedStream.StreamName
				selectedTable.Columns = selectedStream.Columns
				selectedTable.Relations = make([]api2go.TableRelation, 0)

			}

		}

		cols := selectedTable.Columns

		//log.Infof("data: %v", selectedTable.Relations)
		actions, err := cruds["world"].GetActionsByType(typeName)

		if err != nil {
			log.Errorf("Failed to get actions by type: %v", err)
		}

		pr := &http.Request{
			Method: "GET",
		}

		pr = pr.WithContext(c.Request.Context())

		params := make(map[string][]string)
		req := api2go.Request{
			PlainRequest: pr,
			QueryParams:  params,
		}

		worldRefId := worldToReferenceId[typeName]

		params["worldName"] = []string{"smd_id"}
		params["world_id"] = []string{worldRefId}

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
			//log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue)
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
				res[rel.GetObjectName()] = NewJsonApiRelation(rel.GetObject(), rel.GetObjectName(), r, "entity")
			} else {
				if rel.GetRelation() == "belongs_to" {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), rel.GetSubjectName(), "hasMany", "entity")
				} else if rel.GetRelation() == "has_one" {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), rel.GetSubjectName(), "hasMany", "entity")
				} else {
					res[rel.GetSubjectName()] = NewJsonApiRelation(rel.GetSubject(), rel.GetSubjectName(), "hasMany", "entity")
				}
			}
		}
		res["__type"] = api2go.ColumnInfo{
			Name:       "type",
			ColumnName: "__type",
			ColumnType: "hidden",
		}

		jsModel := JsModel{
			ColumnModel:           res,
			Actions:               actions,
			StateMachines:         smdList,
			IsStateMachineEnabled: selectedTable.IsStateTrackingEnabled,
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

type JsModel struct {
	ColumnModel           map[string]interface{}
	Actions               []resource.Action
	StateMachines         []map[string]interface{}
	IsStateMachineEnabled bool
}

func NewJsonApiRelation(name string, relationName string, relationType string, columnType string) JsonApiRelation {

	return JsonApiRelation{
		Type:       name,
		JsonApi:    relationType,
		ColumnType: columnType,
		ColumnName: relationName,
	}

}

type JsonApiRelation struct {
	JsonApi    string `json:"jsonApi,omitempty"`
	ColumnType string `json:"columnType"`
	Type       string `json:"type,omitempty"`
	ColumnName string `json:"ColumnName"`
}
