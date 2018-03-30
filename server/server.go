package server

import (
	"github.com/artpar/api2go"
	"github.com/artpar/api2go-adapter/gingonic"
	"github.com/artpar/go.uuid"
	"github.com/artpar/rclone/cmd"
	"github.com/artpar/rclone/fs"
	"github.com/artpar/rclone/fs/config"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/database"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/websockets"
	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"github.com/thoas/stats"
	"io/ioutil"
	"net/http"
	"github.com/graphql-go/graphql"
	"fmt"
	"encoding/json"
	"github.com/aws/aws-sdk-go/private/util"
	"github.com/gedex/inflector"
	"strings"
	"encoding/base64"
	"github.com/pkg/errors"
)

var Stats = stats.New()

// Capitalize capitalizes the first character of the string.
func Capitalize(s string) string {
	if len(s) == 1 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}

func MakeGraphqlSchema(cmsConfig *resource.CmsConfig, resources map[string]*resource.DbResource) *graphql.Schema {

	graphqlTypesMap := make(map[string]*graphql.Object)
	mutations := make(graphql.Fields)
	query := make(graphql.Fields)

	for _, table := range cmsConfig.Tables {

		fields := make(graphql.Fields)

		for _, column := range table.Columns {

			if column.IsForeignKey {
				continue
			}

			fields[column.ColumnName] = &graphql.Field{
				Type: resource.ColumnManager.GetGraphqlType(column.ColumnType),
				Name: column.Name,
			}
		}
		objectConfig := graphql.NewObject(graphql.ObjectConfig{
			Name:   table.TableName,
			Fields: fields,
		})

		graphqlTypesMap[table.TableName] = objectConfig

	}

	for _, table := range cmsConfig.Tables {

		for _, relation := range table.Relations {
			if relation.Relation == "has_one" || relation.Relation == "belongs_to" {
				if relation.Subject == table.TableName {
					graphqlTypesMap[table.TableName].AddFieldConfig(relation.GetObjectName(), &graphql.Field{
						Type: graphqlTypesMap[relation.GetObject()],
						Name: relation.GetObjectName(),
					})
				} else {
					graphqlTypesMap[table.TableName].AddFieldConfig(relation.GetSubjectName(), &graphql.Field{
						Type: graphqlTypesMap[relation.GetSubject()],
						Name: relation.GetSubjectName(),
					})
				}

			} else {
				if relation.Subject == table.TableName {
					graphqlTypesMap[table.TableName].AddFieldConfig(relation.GetObjectName(), &graphql.Field{
						Name: relation.GetObjectName(),
						Type: graphql.NewList(graphqlTypesMap[relation.GetObject()]),
					})
				} else {
					graphqlTypesMap[table.TableName].AddFieldConfig(relation.GetSubjectName(), &graphql.Field{
						Type: graphql.NewList(graphqlTypesMap[relation.GetSubject()]),
						Name: relation.GetSubjectName(),
					})
				}
			}
		}
	}

	for _, table := range cmsConfig.Tables {

		createFields := make(graphql.FieldConfigArgument)
		uniqueFields := make(graphql.FieldConfigArgument)
		allFields := make(graphql.FieldConfigArgument)

		for _, column := range table.Columns {

			if column.IsForeignKey {
				continue
			}

			allFields[column.ColumnName] = &graphql.ArgumentConfig{
				Type: resource.ColumnManager.GetGraphqlType(column.ColumnType),
			}

			if column.IsUnique || column.IsPrimaryKey {
				uniqueFields[column.ColumnName] = &graphql.ArgumentConfig{
					Type: resource.ColumnManager.GetGraphqlType(column.ColumnType),
				}
			}

			if IsStandardColumn(column.ColumnName) {
				continue
			}

			if column.IsForeignKey {
				continue
			}

			if column.IsNullable {
				createFields[column.ColumnName] = &graphql.ArgumentConfig{
					Type: resource.ColumnManager.GetGraphqlType(column.ColumnType),
				}
			} else {
				createFields[column.ColumnName] = &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(resource.ColumnManager.GetGraphqlType(column.ColumnType)),
				}
			}

		}

		for _, relation := range table.Relations {

			if relation.Relation == "has_one" || relation.Relation == "belongs_to" {
				if relation.Subject == table.TableName {
					allFields[relation.GetObjectName()] = &graphql.ArgumentConfig{
						Type: graphqlTypesMap[relation.GetObject()],
					}
				} else {
					allFields[relation.GetSubjectName()] = &graphql.ArgumentConfig{
						Type: graphqlTypesMap[relation.GetSubject()],
					}
				}

			} else {
				if relation.Subject == table.TableName {
					allFields[relation.GetObjectName()] = &graphql.ArgumentConfig{
						Type: graphql.NewList(graphqlTypesMap[relation.GetObject()]),
					}
				} else {
					allFields[relation.GetSubjectName()] = &graphql.ArgumentConfig{
						Type: graphql.NewList(graphqlTypesMap[relation.GetSubject()]),
					}
				}
			}
		}

		mutations["create"+util.Capitalize(table.TableName)] = &graphql.Field{
			Type:        graphqlTypesMap[table.TableName],
			Description: "Create a new " + table.TableName,
			Args:        createFields,
			Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
				return func(p graphql.ResolveParams) (interface{}, error) {
					log.Printf("create resolve params: %v", p)
					return nil, nil
				}
			}(table),
		}

		mutations["update"+Capitalize(table.TableName)] = &graphql.Field{
			Type:        graphqlTypesMap[table.TableName],
			Description: "Create a new " + table.TableName,
			Args:        createFields,
			Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
				return func(p graphql.ResolveParams) (interface{}, error) {
					log.Printf("create resolve params: %v", p)
					return nil, nil
				}
			}(table),
		}

		query[table.TableName] = &graphql.Field{
			Type:        graphqlTypesMap[table.TableName],
			Description: "Get a single " + table.TableName,
			Args:        uniqueFields,
			Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
				return func(params graphql.ResolveParams) (interface{}, error) {

					log.Printf("Arguments: %v", params.Args)

					filters := make([]resource.Query, 0)

					for keyName, value := range params.Args {

						if _, ok := uniqueFields[keyName]; !ok {
							continue
						}

						query := resource.Query{
							ColumnName: keyName,
							Operator:   "is",
							Value:      value.(string),
						}
						filters = append(filters, query)
					}

					pr := http.Request{
						Method: "GET",
					}
					jsStr, err := json.Marshal(filters)
					req := api2go.Request{
						PlainRequest: &pr,
						QueryParams: map[string][]string{
							"query": {base64.StdEncoding.EncodeToString(jsStr)},
						},
					}

					count, responder, err := resources[table.TableName].PaginatedFindAll(req)

					if count == 0 {
						return nil, errors.New("no such entity")
					}

					model := responder.Result().([]*api2go.Api2GoModel)
					return model[0].Data, err

				}
			}(table),
		}

		query["all"+Capitalize(inflector.Pluralize(table.TableName))] = &graphql.Field{
			Type:        graphql.NewList(graphqlTypesMap[table.TableName]),
			Description: "Get a list of " + inflector.Pluralize(table.TableName),
			Args:        allFields,
			Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {

				return func(params graphql.ResolveParams) (interface{}, error) {
					log.Printf("Arguments: %v", params.Args)

					filters := make([]resource.Query, 0)

					for keyName, value := range params.Args {

						if _, ok := uniqueFields[keyName]; !ok {
							continue
						}

						query := resource.Query{
							ColumnName: keyName,
							Operator:   "is",
							Value:      value.(string),
						}
						filters = append(filters, query)
					}

					pr := http.Request{
						Method: "GET",
					}
					jsStr, err := json.Marshal(filters)
					req := api2go.Request{
						PlainRequest: &pr,
						QueryParams: map[string][]string{
							"query":              {base64.StdEncoding.EncodeToString(jsStr)},
							"included_relations": {"*"},
						},
					}

					count, responder, err := resources[table.TableName].PaginatedFindAll(req)

					if count == 0 {
						return nil, errors.New("no such entity")
					}

					items := responder.Result().([]*api2go.Api2GoModel)

					results := make([]map[string]interface{}, 0)
					for _, item := range items {
						ai := item

						dataMap := ai.Data

						includedMap := make(map[string]interface{})

						for _, includedObject := range ai.Includes {
							id := includedObject.GetID()
							includedMap[id] = includedObject.GetAttributes()
						}

						for _, relation := range table.Relations {
							columnName := relation.GetSubjectName()
							if table.TableName == relation.Subject {
								columnName = relation.GetObjectName()
							}
							referencedObjectId := dataMap[columnName]
							if referencedObjectId == nil {
								continue
							}
							dataMap[columnName] = includedMap[referencedObjectId.(string)]
						}

						results = append(results, dataMap)
					}
					return results, err
				}
			}(table),
		}

		query["meta"+Capitalize(inflector.Pluralize(table.TableName))] = &graphql.Field{
			Type:        graphql.NewList(graphqlTypesMap[table.TableName]),
			Description: "Aggregates for " + inflector.Pluralize(table.TableName),
			Args:        allFields,
			Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {

				return func(params graphql.ResolveParams) (interface{}, error) {
					log.Printf("Arguments: %v", params.Args)

					filters := make([]resource.Query, 0)

					for keyName, value := range params.Args {

						if _, ok := uniqueFields[keyName]; !ok {
							continue
						}

						query := resource.Query{
							ColumnName: keyName,
							Operator:   "is",
							Value:      value.(string),
						}
						filters = append(filters, query)
					}

					pr := http.Request{
						Method: "GET",
					}
					jsStr, err := json.Marshal(filters)
					req := api2go.Request{
						PlainRequest: &pr,
						QueryParams: map[string][]string{
							"query":              {base64.StdEncoding.EncodeToString(jsStr)},
							"included_relations": {"*"},
						},
					}

					count, responder, err := resources[table.TableName].PaginatedFindAll(req)

					if count == 0 {
						return nil, errors.New("no such entity")
					}

					items := responder.Result().([]*api2go.Api2GoModel)

					results := make([]map[string]interface{}, 0)
					for _, item := range items {
						ai := item

						dataMap := ai.Data

						includedMap := make(map[string]interface{})

						for _, includedObject := range ai.Includes {
							id := includedObject.GetID()
							includedMap[id] = includedObject.GetAttributes()
						}

						for _, relation := range table.Relations {
							columnName := relation.GetSubjectName()
							if table.TableName == relation.Subject {
								columnName = relation.GetObjectName()
							}
							referencedObjectId := dataMap[columnName]
							if referencedObjectId == nil {
								continue
							}
							dataMap[columnName] = includedMap[referencedObjectId.(string)]
						}

						results = append(results, dataMap)
					}
					return results, err
				}
			}(table),
		}

	}

	var rootMutation = graphql.NewObject(graphql.ObjectConfig{
		Name:   "RootMutation",
		Fields: mutations,
	});
	var rootQuery = graphql.NewObject(graphql.ObjectConfig{
		Name:   "RootQuery",
		Fields: query,
	})

	// define schema, with our rootQuery and rootMutation
	var schema, _ = graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootQuery,
		Mutation: rootMutation,
	})

	return &schema

}

func IsStandardColumn(s string) bool {
	for _, cols := range resource.StandardColumns {
		if cols.ColumnName == s {
			return true
		}
	}
	return false
}

func executeQuery(query string, schema graphql.Schema) *graphql.Result {
	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: query,
	})
	if len(result.Errors) > 0 {
		fmt.Printf("wrong result, unexpected errors: %v", result.Errors)
	}
	return result
}

func Main(boxRoot http.FileSystem, db database.DatabaseConnection) HostSwitch {

	/// Start system initialise

	log.Infof("Load config files")
	initConfig, errs := LoadConfigFiles()
	if errs != nil {
		for _, err := range errs {
			log.Errorf("Failed to load config file: %v", err)
		}
	}

	existingTables, _ := GetTablesFromWorld(db)

	allTables := MergeTables(existingTables, initConfig.Tables)

	initConfig.Tables = allTables
	config.LoadConfig()
	fs.Config.DryRun = false
	fs.Config.LogLevel = 200
	fs.Config.StatsLogLevel = 200

	resource.CheckRelations(&initConfig)
	resource.CheckAuditTables(&initConfig)
	//AddStateMachines(&initConfig, db)
	tx, errb := db.Beginx()
	//_, errb := db.Exec("begin")
	resource.CheckErr(errb, "Failed to begin transaction")

	resource.CheckAllTableStatus(&initConfig, db, tx)
	resource.CreateRelations(&initConfig, tx)
	resource.CreateUniqueConstraints(&initConfig, tx)
	resource.CreateIndexes(&initConfig, tx)
	resource.UpdateWorldTable(&initConfig, tx)
	resource.UpdateWorldColumnTable(&initConfig, tx)
	errc := tx.Commit()
	resource.CheckErr(errc, "Failed to commit transaction")

	resource.UpdateStateMachineDescriptions(&initConfig, db)
	resource.UpdateExchanges(&initConfig, db)
	resource.UpdateStreams(&initConfig, db)
	resource.UpdateMarketplaces(&initConfig, db)
	resource.UpdateStandardData(&initConfig, db)

	err := resource.UpdateActionTable(&initConfig, db)
	resource.CheckErr(err, "Failed to update action table")

	/// end system initialise

	r := gin.Default()

	r.Use(func() gin.HandlerFunc {
		return func(c *gin.Context) {
			beginning, recorder := Stats.Begin(c.Writer)
			defer Stats.End(beginning, recorder)
			c.Next()
		}
	}())

	r.GET("/statistics", func(c *gin.Context) {
		c.JSON(http.StatusOK, Stats.Data())
	})

	r.Use(CorsMiddlewareFunc)
	r.StaticFS("/static", NewSubPathFs(boxRoot, "/static"))

	r.GET("/favicon.ico", func(c *gin.Context) {

		file, err := boxRoot.Open("favicon.ico")
		if err != nil {
			c.AbortWithStatus(404)
			return
		}

		fileContents, err := ioutil.ReadAll(file)
		if err != nil {
			c.AbortWithStatus(404)
			return
		}
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write favico")
	})

	configStore, err := resource.NewConfigStore(db)
	jwtSecret, err := configStore.GetConfigValueFor("jwt.secret", "backend")

	if err != nil {
		u, _ := uuid.NewV4()
		newSecret := u.String()
		configStore.SetConfigValueFor("jwt.secret", newSecret, "backend")
		jwtSecret = newSecret
	}

	resource.CheckErr(err, "Failed to get config store")
	err = CheckSystemSecrets(configStore)
	resource.CheckErr(err, "Failed to initialise system secrets")

	r.GET("/config", CreateConfigHandler(configStore))

	authMiddleware := auth.NewAuthMiddlewareBuilder(db)
	auth.InitJwtMiddleware([]byte(jwtSecret))
	r.Use(authMiddleware.AuthCheckMiddleware)

	cruds := make(map[string]*resource.DbResource)
	r.GET("/actions", resource.CreateGuestActionListHandler(&initConfig))

	api := api2go.NewAPIWithRouting(
		"api",
		api2go.NewStaticResolver("/"),
		gingonic.New(r),
	)

	ms := BuildMiddlewareSet(&initConfig, &cruds)
	cruds = AddResourcesToApi2Go(api, initConfig.Tables, db, &ms, configStore, cruds)

	rcloneRetries, err := configStore.GetConfigIntValueFor("rclone.retries", "backend")
	if err != nil {
		rcloneRetries = 5
		configStore.SetConfigIntValueFor("rclone.retries", rcloneRetries, "backend")
	}
	cmd.SetRetries(&rcloneRetries)

	streamProcessors := GetStreamProcessors(&initConfig, configStore, cruds)
	AddStreamsToApi2Go(api, streamProcessors, db, &ms, configStore)

	resource.ImportDataFiles(&initConfig, db, cruds)

	hostSwitch := CreateSubSites(&initConfig, db, cruds, authMiddleware)

	hostSwitch.handlerMap["api"] = r
	hostSwitch.handlerMap["dashboard"] = r

	authMiddleware.SetUserCrud(cruds["user"])
	authMiddleware.SetUserGroupCrud(cruds["usergroup"])
	authMiddleware.SetUserUserGroupCrud(cruds["user_user_id_has_usergroup_usergroup_id"])

	fsmManager := resource.NewFsmManager(db, cruds)

	r.GET("/ping", func(c *gin.Context) {
		c.String(200, "pong")
	})

	handler := CreateJsModelHandler(&initConfig, cruds)
	metaHandler := CreateMetaHandler(&initConfig)
	blueprintHandler := CreateApiBlueprintHandler(&initConfig, cruds)
	modelHandler := CreateReclineModelHandler()
	statsHandler := CreateStatsHandler(&initConfig, cruds)
	resource.InitialiseColumnManager()

	graphqlSchema := MakeGraphqlSchema(&initConfig, cruds)
	r.GET("/graphql", func(context *gin.Context) {
		log.Infof("graphql query: %v", context.Query("query"))
		result := executeQuery(context.Query("query"), *graphqlSchema)
		json.NewEncoder(context.Writer).Encode(result)
	})

	r.GET("/jsmodel/:typename", handler)
	r.GET("/stats/:typename", statsHandler)
	r.GET("/meta", metaHandler)
	r.GET("/apispec.raml", blueprintHandler)
	r.GET("/recline_model", modelHandler)
	r.OPTIONS("/jsmodel/:typename", handler)
	r.OPTIONS("/apispec.raml", blueprintHandler)
	r.OPTIONS("/recline_model", modelHandler)

	actionPerformers := GetActionPerformers(&initConfig, configStore, cruds)
	initConfig.ActionPerformers = actionPerformers
	//actionPerforMap := make(map[string]resource.ActionPerformerInterface)
	//for _, actionPerformer := range actionPerformers {
	//	actionPerforMap[actionPerformer.Name()] = actionPerformer
	//}
	//initConfig.ActionPerformers = actionPerforMap

	r.POST("/action/:typename/:actionName", resource.CreatePostActionHandler(&initConfig, configStore, cruds, actionPerformers))
	r.GET("/action/:typename/:actionName", resource.CreatePostActionHandler(&initConfig, configStore, cruds, actionPerformers))

	r.POST("/track/start/:stateMachineId", CreateEventStartHandler(fsmManager, cruds, db))
	r.POST("/track/event/:typename/:objectStateId/:eventName", CreateEventHandler(&initConfig, fsmManager, cruds, db))

	r.POST("/site/content/load", CreateSubSiteContentHandler(&initConfig, cruds, db))
	r.POST("/site/content/store", CreateSubSiteSaveContentHandler(&initConfig, cruds, db))

	//webSocketConnectionHandler := WebSocketConnectionHandlerImpl{}
	//websocketServer := websockets.NewServer("/live", &webSocketConnectionHandler)
	//go websocketServer.Listen(r)

	r.NoRoute(func(c *gin.Context) {
		file, err := boxRoot.Open("index.html")
		resource.CheckErr(err, "Failed to open index.html")
		if err != nil {
			c.AbortWithStatus(500)
			return
		}
		fileContents, err := ioutil.ReadAll(file)
		_, err = c.Writer.Write(fileContents)
		resource.CheckErr(err, "Failed to write index html")
	})

	//r.Run(fmt.Sprintf(":%v", *port))
	CleanUpConfigFiles()

	return hostSwitch

}

func MergeTables(existingTables []resource.TableInfo, initConfigTables []resource.TableInfo) []resource.TableInfo {
	allTables := make([]resource.TableInfo, 0)
	existingTablesMap := make(map[string]bool)

	for j, existableTable := range existingTables {
		existingTablesMap[existableTable.TableName] = true
		var isBeingModified = false
		var indexBeingModified = -1

		for i, newTable := range initConfigTables {
			if newTable.TableName == existableTable.TableName {
				isBeingModified = true
				indexBeingModified = i
				break
			}
		}

		if isBeingModified {
			log.Debugf("Table %s is being modified", existableTable.TableName)
			tableBeingModified := initConfigTables[indexBeingModified]

			if len(tableBeingModified.Columns) > 0 {

				for _, newColumnDef := range tableBeingModified.Columns {
					columnAlreadyExist := false
					colIndex := -1
					for i, existingColumn := range existableTable.Columns {
						//log.Infof("Table column old/new [%v][%v] == [%v][%v] @ %v", tableBeingModified.TableName, newColumnDef.Name, existableTable.TableName, existingColumn.Name, i)
						if existingColumn.Name == newColumnDef.Name || existingColumn.ColumnName == newColumnDef.ColumnName {
							columnAlreadyExist = true
							colIndex = i
							break
						}
					}
					//log.Infof("Decide for table column [%v][%v] @ index: %v [%v]", tableBeingModified.TableName, newColumnDef.Name, colIndex, columnAlreadyExist)
					if columnAlreadyExist {
						//log.Infof("Modifying existing columns[%v][%v] is not supported at present. not sure what would break. and alter query isnt being run currently.", existableTable.TableName, newColumnDef.Name);

						existableTable.Columns[colIndex].DefaultValue = newColumnDef.DefaultValue
						existableTable.Columns[colIndex].ExcludeFromApi = newColumnDef.ExcludeFromApi
						existableTable.Columns[colIndex].IsIndexed = newColumnDef.IsIndexed
						existableTable.Columns[colIndex].IsNullable = newColumnDef.IsNullable
						existableTable.Columns[colIndex].ColumnType = newColumnDef.ColumnType
						existableTable.Columns[colIndex].Options = newColumnDef.Options

					} else {
						existableTable.Columns = append(existableTable.Columns, newColumnDef)
					}

				}

			}
			if len(tableBeingModified.Relations) > 0 {

				existingRelations := existableTable.Relations
				relMap := make(map[string]bool)
				for _, rel := range existingRelations {
					relMap[rel.Hash()] = true
				}

				for _, newRel := range tableBeingModified.Relations {

					_, ok := relMap[newRel.Hash()]
					if !ok {
						existableTable.AddRelation(newRel)
					}
				}
			}
			existableTable.DefaultGroups = tableBeingModified.DefaultGroups
			existableTable.Conformations = tableBeingModified.Conformations
			existableTable.Validations = tableBeingModified.Validations
			existingTables[j] = existableTable
		} else {
			//log.Infof("Table %s is not being modified", existableTable.TableName)
		}
		allTables = append(allTables, existableTable)
	}

	for _, newTable := range initConfigTables {
		if existingTablesMap[newTable.TableName] {
			continue
		}
		allTables = append(allTables, newTable)
	}

	return allTables

}

func NewSubPathFs(system http.FileSystem, s string) http.FileSystem {
	return &SubPathFs{system: system, subPath: s}
}

type SubPathFs struct {
	system  http.FileSystem
	subPath string
}

func (spf *SubPathFs) Open(name string) (http.File, error) {
	//log.Infof("Service file from static path: %s/%s", spf.subPath, name)
	return spf.system.Open(spf.subPath + name)
}

type WebSocketConnectionHandlerImpl struct {
}

func (wsch *WebSocketConnectionHandlerImpl) MessageFromClient(message websockets.WebSocketPayload, request *http.Request) {

}

func AddStreamsToApi2Go(api *api2go.API, processors []*resource.StreamProcessor, db database.DatabaseConnection, middlewareSet *resource.MiddlewareSet, configStore *resource.ConfigStore) {

	for _, processor := range processors {

		contract := processor.GetContract()
		model := api2go.NewApi2GoModel(contract.StreamName, contract.Columns, 0, nil)
		api.AddResource(model, processor)

	}

}

func GetStreamProcessors(config *resource.CmsConfig, store *resource.ConfigStore, cruds map[string]*resource.DbResource) []*resource.StreamProcessor {

	allProcessors := make([]*resource.StreamProcessor, 0)

	for _, streamContract := range config.Streams {

		streamProcessor := resource.NewStreamProcessor(streamContract, cruds)
		allProcessors = append(allProcessors, streamProcessor)

	}

	return allProcessors

}
