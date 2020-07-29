package server

import (
	"fmt"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/apiblueprint"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"image/color"
	"net/http"
	"strings"
)

func CreateApiBlueprintHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(ctx *gin.Context) {
	return func(c *gin.Context) {
		c.String(200, "%s", apiblueprint.BuildApiBlueprint(initConfig, cruds))
	}
}

type ErrorResponse struct {
	Message string
}

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		// Double the hex digits:
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid length, must be 7 or 4")

	}
	return
}

func CreateStatsHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {

	return func(c *gin.Context) {

		typeName := c.Param("typename")

		user := c.Request.Context().Value("user")
		var sessionUser *auth.SessionUser
		if user != nil {
			sessionUser = user.(*auth.SessionUser)
		}

		perm := cruds[typeName].GetObjectPermissionByWhereClause("world", "table_name", typeName)
		if sessionUser == nil || !perm.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups) {
			c.AbortWithStatus(403)
			return
		}

		aggReq := resource.AggregationRequest{}

		aggReq.RootEntity = typeName
		aggReq.Filter = c.QueryArray("filter")
		aggReq.GroupBy = c.QueryArray("group")
		aggReq.Join = c.QueryArray("join")
		aggReq.ProjectColumn = c.QueryArray("column")
		aggReq.TimeSample = resource.TimeStamp(c.Query("timesample"))
		aggReq.TimeFrom = c.Query("timefrom")
		aggReq.TimeTo = c.Query("timeto")
		aggReq.Order = c.QueryArray("order")

		aggResponse, err := cruds[typeName].DataStats(aggReq)

		if err != nil {
			c.JSON(500, resource.NewDaptinError("Failed to query stats", "query failed"))
			return
		}

		c.JSON(200, aggResponse)

	}

}

func CreateReclineModelHandler() func(*gin.Context) {

	reclineColumnMap := make(map[string]string)

	for _, column := range resource.ColumnTypes {
		reclineColumnMap[column.Name] = column.ReclineType
	}

	return func(c *gin.Context) {
		c.JSON(200, reclineColumnMap)
	}

}

func CreateMetaHandler(initConfig *resource.CmsConfig) func(*gin.Context) {

	return func(context *gin.Context) {

		query := context.Query("query")

		switch query {
		case "column_types":
			context.JSON(200, resource.ColumnManager.ColumnMap)
		}
	}
}

func CreateJsModelHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource) func(*gin.Context) {
	tableMap := make(map[string]resource.TableInfo)
	for _, table := range initConfig.Tables {

		//log.Infof("Default permission for [%v]: [%v]", table.TableName, table.Columns)

		tableMap[table.TableName] = table
	}

	streamMap := make(map[string]resource.StreamContract)
	for _, stream := range initConfig.Streams {
		streamMap[stream.StreamName] = stream
	}

	worlds, _, err := cruds["world"].GetRowsByWhereClause("world")
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
			log.Infof("%v is not a table", typeName)
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
			//log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue, col.IsForeignKey, col.ForeignKeyData)
			if col.ExcludeFromApi {
				continue
			}

			if col.IsForeignKey && col.ForeignKeyData.DataSource == "self" {
				continue
			}

			res[col.ColumnName] = col
			if col.ColumnName == "reference_id" {
				res["relation_reference_id"] = col
			}
		}

		for _, rel := range selectedTable.Relations {
			//log.Infof("Relation [%v][%v]", selectedTable.TableName, rel.String())

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

		for _, col := range cols {
			//log.Infof("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue)
			if col.ExcludeFromApi {
				continue
			}

			if !col.IsForeignKey || col.ForeignKeyData.DataSource == "self" {
				continue
			}

			//res[col.ColumnName] = NewJsonApiRelation(col.Name, col.ColumnName, "hasOne", col.ColumnType)
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
