package server

import (
	"crypto/sha256"
	"fmt"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/apiblueprint"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
	"image/color"
	"net/http"
	"strings"
	"sync"
	"time"
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

		if cruds[typeName] == nil {
			log.Errorf("entity not found for aggregation: %v", typeName)
			c.AbortWithStatus(404)
			return
		}

		transaction, err := cruds[typeName].Connection().Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [65]")
			return
		}

		defer transaction.Rollback()

		perm := cruds[typeName].GetObjectPermissionByWhereClause("world", "table_name", typeName, transaction)
		if sessionUser == nil || !perm.CanExecute(sessionUser.UserReferenceId, sessionUser.Groups, cruds["usergroup"].AdministratorGroupId) {
			log.Infof("user [%v] not allowed to execute aggregate on [%v]", sessionUser, typeName)
			c.AbortWithStatus(403)
			return
		}

		aggReq := resource.AggregationRequest{}
		if c.Request.Method == "POST" {
			err = c.Bind(&aggReq)
			if err != nil {
				log.Errorf("Error parsing aggregation request: %v", err)
				c.AbortWithStatus(400)
				return
			}
			aggReq.RootEntity = typeName
		}

		if c.Request.Method == "GET" {
			aggReq.RootEntity = typeName
			aggReq.Filter = c.QueryArray("filter")
			aggReq.Having = c.QueryArray("having")
			aggReq.GroupBy = c.QueryArray("group")
			aggReq.Join = c.QueryArray("join")
			aggReq.ProjectColumn = c.QueryArray("column")
			aggReq.TimeSample = resource.TimeStamp(c.Query("timesample"))
			aggReq.TimeFrom = c.Query("timefrom")
			aggReq.TimeTo = c.Query("timeto")
			aggReq.Order = c.QueryArray("order")
		}

		aggResponse, err := cruds[typeName].DataStats(aggReq, transaction)

		if err != nil {
			log.Errorf("failed to execute aggregation [%v] - %v", typeName, err)
			c.JSON(500, resource.NewDaptinError("Failed to query stats", "query failed - "+err.Error()))
			return
		}

		c.JSON(200, aggResponse)

	}

}

func CreateMetaHandler(initConfig *resource.CmsConfig) func(*gin.Context) {
	columnTypesMap := resource.ColumnManager.ColumnMap
	columnTypesResponse, _ := json.MarshalToString(columnTypesMap)
	columnTypesResponseEtag := fmt.Sprintf("W/\"%x\"", sha256.Sum256([]byte(columnTypesResponse)))

	return func(context *gin.Context) {
		// Set aggressive cache control headers
		context.Header("Cache-Control", "public, max-age=86400, s-maxage=86400, immutable")
		context.Header("Expires", time.Now().Add(24*time.Hour).Format(http.TimeFormat))
		context.Header("Pragma", "cache")

		query := context.Query("query")

		switch query {
		case "column_types":
			// Check if browser sent If-None-Match header (ETag)
			context.Header("ETag", columnTypesResponseEtag)

			if match := context.GetHeader("If-None-Match"); match != "" {
				if strings.Contains(match, columnTypesResponseEtag) {
					context.Status(http.StatusNotModified)
					return
				}
			}

			context.String(200, columnTypesResponse)
		}
	}
}

func CreateJsModelHandler(initConfig *resource.CmsConfig, cruds map[string]*resource.DbResource, transaction *sqlx.Tx) func(*gin.Context) {
	tableMap := make(map[string]table_info.TableInfo)
	for _, table := range initConfig.Tables {

		//log.Printf("Default permission for [%v]: [%v]", table.TableName, table.Columns)

		tableMap[table.TableName] = table
	}

	streamMap := make(map[string]resource.StreamContract)
	for _, stream := range initConfig.Streams {
		streamMap[stream.StreamName] = stream
	}

	worlds, _, err := cruds["world"].GetRowsByWhereClause("world", nil, transaction)
	if err != nil {
		log.Errorf("Failed to get worlds list")
	}

	worldToReferenceId := make(map[string]daptinid.DaptinReferenceId)

	for _, world := range worlds {
		worldToReferenceId[world["table_name"].(string)] = daptinid.InterfaceToDIR(world["reference_id"])
	}
	var cacheMap sync.Map
	//cacheMap := make(map[string]string)

	return func(c *gin.Context) {
		typeName := strings.Split(c.Param("typename"), ".")[0]
		if jsModel, ok := cacheMap.Load(typeName); ok {
			c.String(200, jsModel.(string))
			return
		}
		selectedTable, isTable := tableMap[typeName]

		if !isTable {
			log.Printf("%v is not a table", typeName)
			selectedStream, isStream := streamMap[typeName]

			if !isStream {
				c.AbortWithStatus(404)
				return

			} else {
				selectedTable = table_info.TableInfo{}
				selectedTable.TableName = selectedStream.StreamName
				selectedTable.Columns = selectedStream.Columns
				selectedTable.Relations = make([]api2go.TableRelation, 0)

			}

		}

		cols := selectedTable.Columns

		//log.Printf("data: %v", selectedTable.Relations)
		tx, err := cruds["world"].Connection().Beginx()
		if err != nil {
			resource.CheckErr(err, "Failed to begin transaction [170]")
			return
		}

		defer tx.Rollback()
		actions, err := cruds["world"].GetActionsByType(typeName, tx)

		if err != nil {
			log.Errorf("Failed to get actions by type: %v", err)
		}

		pr := &http.Request{
			Method: "GET",
			URL:    c.Request.URL,
		}

		pr = pr.WithContext(c.Request.Context())

		params := make(map[string][]string)
		req := api2go.Request{
			PlainRequest: pr,
			QueryParams:  params,
		}

		worldRefId := worldToReferenceId[typeName]

		params["worldName"] = []string{"smd_id"}
		params["world_id"] = []string{worldRefId.String()}

		smdList := make([]map[string]interface{}, 0)

		_, result, err := cruds["smd"].PaginatedFindAllWithTransaction(req, tx)

		if err != nil {
			log.Printf("Failed to get world SMD: %v", err)
		} else {
			models := result.Result().([]api2go.Api2GoModel)
			for _, m := range models {
				if m.GetAttributes()["__type"].(string) == "smd" {
					smdList = append(smdList, m.GetAttributes())
				}
			}

		}

		res := map[string]interface{}{}

		for _, col := range cols {
			//log.Printf("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue, col.IsForeignKey, col.ForeignKeyData)
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
			//log.Printf("Relation [%v][%v]", selectedTable.TableName, rel.String())

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
			//log.Printf("Column [%v] default value [%v]", col.ColumnName, col.DefaultValue)
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

		for i, action := range actions {
			action.OutFields = nil
			actions[i] = action
		}

		jsModel := JsModel{
			ColumnModel:           res,
			Actions:               actions,
			StateMachines:         smdList,
			IsStateMachineEnabled: selectedTable.IsStateTrackingEnabled,
		}

		//res["__type"] = "string"

		resBody, err := json.Marshal(jsModel)
		if err != nil {
			c.Error(err)
			return
		}
		asStr := string(resBody)
		cacheMap.Store(typeName, asStr)
		c.String(200, asStr)

	}
}

type JsModel struct {
	ColumnModel           map[string]interface{}
	Actions               []actionresponse.Action
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
