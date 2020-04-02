package server

import (
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/resource"
	"github.com/gobuffalo/flect"
	"github.com/graphql-go/graphql"
	"github.com/graphql-go/relay"
	"github.com/iancoleman/strcase"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"net/http"
	"strings"
	//	"encoding/base64"
	"errors"
	"github.com/json-iterator/go"
	//"fmt"
	"fmt"
	"github.com/artpar/api2go/jsonapi"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

var nodeDefinitions *relay.NodeDefinitions

var Schema graphql.Schema

func MakeGraphqlSchema(cmsConfig *resource.CmsConfig, resources map[string]*resource.DbResource) *graphql.Schema {

	//mutations := make(graphql.InputObjectConfigFieldMap)
	//query := make(graphql.InputObjectConfigFieldMap)
	//done := make(map[string]bool)

	inputTypesMap := make(map[string]*graphql.Object)
	//outputTypesMap := make(map[string]graphql.Output)
	//connectionMap := make(map[string]*relay.GraphQLConnectionDefinitions)

	nodeDefinitions = relay.NewNodeDefinitions(relay.NodeDefinitionsConfig{
		IDFetcher: func(id string, info graphql.ResolveInfo, ctx context.Context) (interface{}, error) {
			resolvedID := relay.FromGlobalID(id)
			pr := &http.Request{
				Method: "GET",
			}
			pr = pr.WithContext(ctx)
			req := api2go.Request{
				PlainRequest: pr,
			}
			responder, err := resources[strings.ToLower(resolvedID.Type)].FindOne(resolvedID.ID, req)
			if responder != nil && responder.Result() != nil {
				return responder.Result().(api2go.Api2GoModel).Data, err
			}
			return nil, err
		},
		TypeResolve: func(p graphql.ResolveTypeParams) *graphql.Object {
			log.Printf("Type resolve query: %v", p)
			//return inputTypesMap[p.Value]
			return nil
		},
	})

	rootFields := make(graphql.Fields)
	mutationFields := make(graphql.Fields)

	actionResponseType := graphql.NewObject(graphql.ObjectConfig{
		Name:        "ActionResponse",
		Description: "Action response",
		Fields: graphql.Fields{
			"ResponseType": &graphql.Field{
				Type: graphql.String,
			},
			"Attributes": &graphql.Field{
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "Attributes",
					Fields: graphql.Fields{
						"type": &graphql.Field{
							Name: "type",
							Type: graphql.String,
						},
						"message": &graphql.Field{
							Name: "message",
							Type: graphql.String,
						},
						"key": &graphql.Field{
							Name: "key",
							Type: graphql.String,
						},
						"value": &graphql.Field{
							Name: "value",
							Type: graphql.String,
						},
						"token": &graphql.Field{
							Name: "token",
							Type: graphql.String,
						},
						"title": &graphql.Field{
							Name: "title",
							Type: graphql.String,
						},
						"delay": &graphql.Field{
							Name: "delay",
							Type: graphql.String,
						},
						"location": &graphql.Field{
							Name: "location",
							Type: graphql.String,
						},
					},
				}),
			},
		},
	})

	pageConfig := graphql.ArgumentConfig{
		Type: graphql.NewInputObject(graphql.InputObjectConfig{
			Name:        "page",
			Description: "Page size and number",
			Fields: graphql.InputObjectConfigFieldMap{
				"number": &graphql.InputObjectFieldConfig{
					Type:         graphql.Int,
					DefaultValue: 1,
					Description:  "page number to fetch",
				},
				"size": &graphql.InputObjectFieldConfig{
					Type:         graphql.Int,
					DefaultValue: 10,
					Description:  "number of records in one page",
				},
			},
		}),
		Description:  "filter results by search query",
		DefaultValue: "",
	}

	queryArgument := graphql.ArgumentConfig{
		Type: graphql.NewList(graphql.NewInputObject(graphql.InputObjectConfig{
			Name:        "query",
			Description: "query results",
			Fields: graphql.InputObjectConfigFieldMap{
				"column": &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				},
				"operator": &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				},
				"value": &graphql.InputObjectFieldConfig{
					Type: graphql.String,
				},
			},
		})),
		Description:  "filter results by search query",
		DefaultValue: "",
	}

	filterArgument := graphql.ArgumentConfig{
		Type:         graphql.String,
		Description:  "filter data by keyword",
		DefaultValue: "",
	}

	for _, table := range cmsConfig.Tables {
		if table.IsJoinTable {
			continue
		}

		tableType := graphql.NewObject(graphql.ObjectConfig{
			Name: table.TableName,
			Interfaces: []*graphql.Interface{
				nodeDefinitions.NodeInterface,
			},
			Fields:      graphql.Fields{},
			Description: table.TableName,
		})

		inputTypesMap[table.TableName] = tableType

	}

	tableColumnMap := map[string]map[string]api2go.ColumnInfo{}

	for _, table := range cmsConfig.Tables {
		if table.IsJoinTable {
			continue
		}
		columnMap := map[string]api2go.ColumnInfo{}

		for _, col := range table.Columns {
			columnMap[col.ColumnName] = col
		}
		tableColumnMap[table.TableName] = columnMap
	}

	for _, table := range cmsConfig.Tables {

		if table.IsJoinTable {
			continue
		}
		allFields := make(graphql.FieldConfigArgument)
		uniqueFields := make(graphql.FieldConfigArgument)

		fields := make(graphql.Fields)

		for _, column := range table.Columns {

			allFields[table.TableName+"."+column.ColumnName] = &graphql.ArgumentConfig{
				Type:         resource.ColumnManager.GetGraphqlType(column.ColumnType),
				DefaultValue: column.DefaultValue,
				Description:  column.ColumnDescription,
			}

			if column.IsUnique || column.IsPrimaryKey {
				uniqueFields[table.TableName+"."+column.ColumnName] = &graphql.ArgumentConfig{
					Type:         resource.ColumnManager.GetGraphqlType(column.ColumnType),
					DefaultValue: column.DefaultValue,
					Description:  column.ColumnDescription,
				}
			}

			if column.IsForeignKey {
				continue
			}

			var graphqlType graphql.Type
			//if column.IsForeignKey {
			//	switch column.ForeignKeyData.DataSource {
			//	case "self":
			//		graphqlType = inputTypesMap[column.ForeignKeyData.Namespace]
			//	case "cloud_store":
			//		graphqlType = inputTypesMap[column.ForeignKeyData.Namespace]
			//	default:
			//		log.Errorf("Unknown data source of column [%s] in table [%v] cannot be defined in graphql schema %s", column.ColumnName, table.TableName, column.ForeignKeyData)
			//	}
			//} else {
			graphqlType = resource.ColumnManager.GetGraphqlType(column.ColumnType)
			//}

			fields[column.ColumnName] = &graphql.Field{
				Type:        graphqlType,
				Description: column.ColumnDescription,
			}
		}

		for _, relation := range table.Relations {

			targetName := relation.GetSubjectName()
			targetObject := relation.GetSubject()
			if relation.Subject == table.TableName {
				targetName = relation.GetObjectName()
				targetObject = relation.GetObject()
			}

			switch relation.Relation {
			case "belongs_to":
				fields[targetName] = &graphql.Field{
					Type:        graphql.NewNonNull(inputTypesMap[targetObject]),
					Description: fmt.Sprintf("Belongs to %v", relation.Subject),
				}
			case "has_one":
				fields[targetName] = &graphql.Field{
					Type:        inputTypesMap[targetObject],
					Description: fmt.Sprintf("Has one %v", relation.Subject),
				}

			case "has_many":
				fields[targetName] = &graphql.Field{
					Type:        graphql.NewList(inputTypesMap[targetObject]),
					Description: fmt.Sprintf("Has many %v", relation.Subject),
				}

			case "has_many_and_belongs_to_many":
				fields[targetName] = &graphql.Field{
					Type:        graphql.NewList(inputTypesMap[targetObject]),
					Description: fmt.Sprintf("Related %v", relation.Subject),
				}

			}

		}

		fields["id"] = &graphql.Field{
			Description: "The ID of an object",
			Type:        graphql.NewNonNull(graphql.ID),
		}

		for fieldName, config := range fields {
			inputTypesMap[table.TableName].AddFieldConfig(fieldName, config)
		}

		// all table names query field

		rootFields[table.TableName] = &graphql.Field{
			Type:        graphql.NewList(inputTypesMap[table.TableName]),
			Description: "Find all " + table.TableName,
			Args: graphql.FieldConfigArgument{
				"filter": &filterArgument,
				"query":  &queryArgument,
				"page":   &pageConfig,
			},
			//Args:        uniqueFields,
			Resolve: func(table resource.TableInfo) func(params graphql.ResolveParams) (interface{}, error) {
				return func(params graphql.ResolveParams) (interface{}, error) {

					log.Printf("Arguments: %v", params.Args)

					filters := make([]resource.Query, 0)

					query, isQueried := params.Args["query"]
					if isQueried {
						queryMap, ok := query.([]interface{})
						if ok {
							for _, qu := range queryMap {
								q := qu.(map[string]interface{})
								query := resource.Query{
									ColumnName: q["column"].(string),
									Operator:   q["operator"].(string),
									Value:      q["value"].(string),
								}
								filters = append(filters, query)
							}
						}
					}

					filter, isFiltered := params.Args["filter"]

					if !isFiltered {
						filter = ""
					}

					pr := &http.Request{
						Method: "GET",
					}
					pr = pr.WithContext(params.Context)

					pageNumber := 1
					pageSize := 10
					pageParams, ok := params.Args["page"]
					if ok {
						pageParamsMap, ok := pageParams.(map[string]interface{})
						if ok {
							pageSizeNew, ok := pageParamsMap["size"]
							if ok {
								pageSize, ok = pageSizeNew.(int)
							}
							pageNumberNew, ok := pageParamsMap["number"]
							if ok {
								pageNumber, ok = pageNumberNew.(int)
							}
						}

					}

					jsStr, err := json.Marshal(filters)
					req := api2go.Request{
						PlainRequest: pr,

						QueryParams: map[string][]string{
							"query":              {string(jsStr)},
							"filter":             {filter.(string)},
							"page[number]":       {fmt.Sprintf("%v", pageNumber)},
							"page[size]":         {fmt.Sprintf("%v", pageSize)},
							"included_relations": {"*"},
						},
					}

					count, responder, err := resources[table.TableName].PaginatedFindAll(req)

					if count == 0 {
						return nil, errors.New("no such entity")
					}
					items := make([]map[string]interface{}, 0)

					results := responder.Result().([]*api2go.Api2GoModel)

					columnMap := tableColumnMap[table.TableName]

					for _, r := range results {

						included := r.Includes
						includedMap := make(map[string]jsonapi.MarshalIdentifier)

						for _, included := range included {
							includedMap[included.GetID()] = included
						}

						data := r.Data

						for key, val := range data {
							colInfo, ok := columnMap[key]
							if !ok {
								continue
							}

							strVal, ok := val.(string)
							if !ok {
								continue
							}

							if colInfo.IsForeignKey {
								fObj, ok := includedMap[strVal]

								if ok {
									data[key] = fObj.GetAttributes()
								}
							}

						}

						items = append(items, data)

					}

					return items, err

				}
			}(table),
		}
		//
		//rootFields["all"+Capitalize(inflector.Pluralize(table.TableName))] = &graphql.Field{
		//	Type:        graphql.NewList(inputTypesMap[table.TableName]),
		//	Description: "Get a list of " + inflector.Pluralize(table.TableName),
		//	Args:        allFields,
		//	Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
		//
		//		return func(params graphql.ResolveParams) (interface{}, error) {
		//			log.Printf("Arguments: %v", params.Args)
		//
		//			filters := make([]resource.Query, 0)
		//
		//			for keyName, value := range params.Args {
		//
		//				if _, ok := uniqueFields[keyName]; !ok {
		//					continue
		//				}
		//
		//				query := resource.Query{
		//					ColumnName: keyName,
		//					Operator:   "is",
		//					Value:      value.(string),
		//				}
		//				filters = append(filters, query)
		//			}
		//
		//			pr := &http.Request{
		//				Method: "GET",
		//			}
		//			pr = pr.WithContext(params.Context)
		//			jsStr, err := json.Marshal(filters)
		//			req := api2go.Request{
		//				PlainRequest: pr,
		//				QueryParams: map[string][]string{
		//					"query":              {base64.StdEncoding.EncodeToString(jsStr)},
		//					"included_relations": {"*"},
		//				},
		//			}
		//
		//			count, responder, err := resources[table.TableName].PaginatedFindAll(req)
		//
		//			if count == 0 {
		//				return nil, errors.New("no such entity")
		//			}
		//
		//			items := responder.Result().([]*api2go.Api2GoModel)
		//
		//			results := make([]map[string]interface{}, 0)
		//			for _, item := range items {
		//				ai := item
		//
		//				dataMap := ai.Data
		//
		//				includedMap := make(map[string]interface{})
		//
		//				for _, includedObject := range ai.Includes {
		//					id := includedObject.GetID()
		//					includedMap[id] = includedObject.GetAttributes()
		//				}
		//
		//				for _, relation := range table.Relations {
		//					columnName := relation.GetSubjectName()
		//					if table.TableName == relation.Subject {
		//						columnName = relation.GetObjectName()
		//					}
		//					referencedObjectId := dataMap[columnName]
		//					if referencedObjectId == nil {
		//						continue
		//					}
		//					dataMap[columnName] = includedMap[referencedObjectId.(string)]
		//				}
		//
		//				results = append(results, dataMap)
		//			}
		//			return results, err
		//		}
		//	}(table),
		//}
		//
		//rootFields["meta"+Capitalize(inflector.Pluralize(table.TableName))] = &graphql.Field{
		//	Type:        graphql.NewList(graphql.NewObject(graphql.ObjectConfig{
		//		//Name
		//	})),
		//	Description: "Aggregates for " + inflector.Pluralize(table.TableName),
		//	Args: graphql.FieldConfigArgument{
		//		"group": &graphql.ArgumentConfig{
		//			Type: graphql.NewList(graphql.String),
		//		},
		//		"join": &graphql.ArgumentConfig{
		//			Type: graphql.NewList(graphql.String),
		//		},
		//		"column": &graphql.ArgumentConfig{
		//			Type: graphql.NewList(graphql.String),
		//		},
		//		"order": &graphql.ArgumentConfig{
		//			Type: graphql.NewList(graphql.String),
		//		},
		//	},
		//	Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
		//
		//		return func(params graphql.ResolveParams) (interface{}, error) {
		//			log.Printf("Arguments: %v", params.Args)
		//			aggReq := resource.AggregationRequest{}
		//
		//			aggReq.RootEntity = table.TableName
		//
		//			if params.Args["group"] != nil {
		//				groupBys := params.Args["group"].([]interface{})
		//				aggReq.GroupBy = make([]string, 0)
		//				for _, grp := range groupBys {
		//					aggReq.GroupBy = append(aggReq.GroupBy, grp.(string))
		//				}
		//			}
		//			if params.Args["join"] != nil {
		//				groupBys := params.Args["join"].([]interface{})
		//				aggReq.Join = make([]string, 0)
		//				for _, grp := range groupBys {
		//					aggReq.Join = append(aggReq.Join, grp.(string))
		//				}
		//			}
		//			if params.Args["column"] != nil {
		//				groupBys := params.Args["column"].([]interface{})
		//				aggReq.ProjectColumn = make([]string, 0)
		//				for _, grp := range groupBys {
		//					aggReq.ProjectColumn = append(aggReq.ProjectColumn, grp.(string))
		//				}
		//			}
		//			if params.Args["order"] != nil {
		//				groupBys := params.Args["order"].([]interface{})
		//				aggReq.Order = make([]string, 0)
		//				for _, grp := range groupBys {
		//					aggReq.Order = append(aggReq.Order, grp.(string))
		//				}
		//			}
		//
		//			//params.Args["query"].(string)
		//			//aggReq.Query =
		//
		//			aggResponse, err := resources[table.TableName].DataStats(aggReq)
		//			return aggResponse, err
		//		}
		//	}(table),
		//}
	}
	rootFields["node"] = nodeDefinitions.NodeField

	//rootQuery := graphql.NewObject(graphql.ObjectConfig{
	//	Name:   "RootQuery",
	//	Fields: rootFields,
	//})

	// root query
	// we just define a trivial example here, since root query is required.
	// Test with curl
	// curl -g 'http://localhost:8080/graphql?query={lastTodo{id,text,done}}'
	var rootQuery = graphql.NewObject(graphql.ObjectConfig{
		Name:   "RootQuery",
		Fields: rootFields,
	})

	for _, t := range cmsConfig.Tables {
		if t.IsJoinTable {
			continue
		}

		func(table resource.TableInfo) {

			inputFields := make(graphql.FieldConfigArgument)
			updateFields := make(graphql.FieldConfigArgument)

			for _, col := range table.Columns {

				if resource.IsStandardColumn(col.ColumnName) {
					continue
				}
				if col.IsForeignKey {
					continue
				}

				var finalGraphqlType graphql.Type
				var finalGraphqlType1 graphql.Type
				finalGraphqlType = resource.ColumnManager.GetGraphqlType(col.ColumnType)
				finalGraphqlType1 = finalGraphqlType

				updateFields[col.ColumnName] = &graphql.ArgumentConfig{
					Type:         finalGraphqlType,
					Description:  col.ColumnDescription,
					DefaultValue: col.DefaultValue,
				}

				if !col.IsNullable || col.ColumnType == "encrypted" {
					finalGraphqlType1 = graphql.NewNonNull(finalGraphqlType)
				}

				inputFields[col.ColumnName] = &graphql.ArgumentConfig{
					Type:         finalGraphqlType1,
					Description:  col.ColumnDescription,
					DefaultValue: col.DefaultValue,
				}

			}

			mutationFields["add"+strcase.ToCamel(table.TableName)] = &graphql.Field{
				Type:        inputTypesMap[table.TableName],
				Description: "Create new " + strings.ReplaceAll(table.TableName, "_", " "),
				Args:        inputFields,
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					obj := api2go.NewApi2GoModelWithData(table.TableName, nil, 0, nil, params.Args)

					pr := &http.Request{
						Method: "POST",
					}

					pr = pr.WithContext(params.Context)

					req := api2go.Request{
						PlainRequest: pr,
					}

					created, err := resources[table.TableName].Create(obj, req)

					if err != nil {
						return nil, err
					}

					return created.Result().(*api2go.Api2GoModel).Data, err
				},
			}

			updateInputFields := make(graphql.FieldConfigArgument)
			for k, v := range updateFields {
				updateInputFields[k] = v
			}

			updateInputFields["resource_id"] = &graphql.ArgumentConfig{
				Type:        graphql.NewNonNull(graphql.String),
				Description: "Resource id",
			}

			mutationFields["update"+strcase.ToCamel(table.TableName)] = &graphql.Field{
				Type:        inputTypesMap[table.TableName],
				Description: "Update " + strings.ReplaceAll(table.TableName, "_", " "),
				Args:        updateInputFields,
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {

					resourceId := params.Args["resource_id"].(string)

					sessionUser := &auth.SessionUser{}
					sessionUserInterface := params.Context.Value("user")
					if sessionUserInterface != nil {
						sessionUser = sessionUserInterface.(*auth.SessionUser)
					}

					existingObj, _, err := resources[table.TableName].GetSingleRowByReferenceId(table.TableName, resourceId, nil)
					if err != nil {
						return nil, err
					}

					permission := resources[table.TableName].GetRowPermission(existingObj)

					if !permission.CanPeek(sessionUser.UserReferenceId, sessionUser.Groups) {
						return nil, errors.New("unauthorized")
					}

					obj := api2go.NewApi2GoModelWithData(table.TableName, nil, 0, nil, existingObj)

					args := params.Args
					deleteKeys := make([]string, 0)
					for k := range args {
						if args[k] == "" {
							deleteKeys = append(deleteKeys, k)
						}
					}

					for _, s := range deleteKeys {
						delete(args, s)
					}

					obj.SetAttributes(args)

					pr := &http.Request{
						Method: "PATCH",
					}

					pr = pr.WithContext(params.Context)

					req := api2go.Request{
						PlainRequest: pr,
					}

					created, err := resources[table.TableName].Update(obj, req)

					if err != nil {
						return nil, err
					}

					return created.Result().(*api2go.Api2GoModel).Data, err
				},
			}

			mutationFields["delete"+strcase.ToCamel(table.TableName)] = &graphql.Field{
				Type:        inputTypesMap[table.TableName],
				Description: "Delete " + strings.ReplaceAll(table.TableName, "_", " "),
				Args: graphql.FieldConfigArgument{
					"resource_id": &graphql.ArgumentConfig{
						Type:        graphql.String,
						Description: "Resource id",
					},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {

					pr := &http.Request{
						Method: "DELETE",
					}

					pr = pr.WithContext(params.Context)

					req := api2go.Request{
						PlainRequest: pr,
					}

					_, err := resources[table.TableName].Delete(params.Args["resource_id"].(string), req)

					if err != nil {
						return nil, err
					}

					return fmt.Sprintf(`{
													"data": {
														"delete%s": {
														}
													}
												}`, flect.Capitalize(table.TableName)), err
				},
			}

		}(t)

	}

	for _, a := range cmsConfig.Actions {

		func(action resource.Action) {

			inputFields := make(graphql.FieldConfigArgument)

			for _, col := range action.InFields {

				var finalGraphqlType graphql.Type
				finalGraphqlType = resource.ColumnManager.GetGraphqlType(col.ColumnType)

				if !col.IsNullable {
					finalGraphqlType = graphql.NewNonNull(finalGraphqlType)
				}

				inputFields[col.ColumnName] = &graphql.ArgumentConfig{
					Type:         finalGraphqlType,
					Description:  col.ColumnDescription,
					DefaultValue: col.DefaultValue,
				}

			}

			//if !action.InstanceOptional {
			//	inputFields[action.OnType+"_id"] = &graphql.ArgumentConfig{
			//		Type:        graphql.NewNonNull(graphql.String),
			//		Description: "reference id of subject " + action.OnType,
			//	}
			//}

			mutationFields["execute"+strcase.ToCamel(action.Name)+"On"+strcase.ToCamel(action.OnType)] = &graphql.Field{
				Type:        graphql.NewList(actionResponseType),
				Description: "Execute " + strings.ReplaceAll(action.Name, "_", " ") + " on " + action.OnType,
				Args:        inputFields,
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {

					pr := &http.Request{
						Method: "EXECUTE",
					}

					pr = pr.WithContext(params.Context)

					req := api2go.Request{
						PlainRequest: pr,
					}

					actionRequest := resource.ActionRequest{
						Type:       action.OnType,
						Action:     action.Name,
						Attributes: params.Args,
					}

					response, err := resources[action.OnType].HandleActionRequest(&actionRequest, req)

					return response, err
				},
			}
		}(a)

	}

	//changeTodoStatusMutation := relay.MutationWithClientMutationID(relay.MutationConfig{
	//	Name: "ChangeTodoStatus",
	//	InputFields: graphql.InputObjectConfigFieldMap{
	//		"id": &graphql.InputObjectFieldConfig{
	//			Type: graphql.NewNonNull(graphql.ID),
	//		},
	//		"complete": &graphql.InputObjectFieldConfig{
	//			Type: graphql.NewNonNull(graphql.Boolean),
	//		},
	//	},
	//	OutputFields: graphql.Fields{
	//		"todo": &graphql.Field{
	//			Type: todoType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				//payload, _ := p.Source.(map[string]interface{})
	//				//todoId, _ := payload["todoId"].(string)
	//				//todo := nil
	//				return nil, nil
	//			},
	//		},
	//		"viewer": &graphql.Field{
	//			Type: userType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				return nil, nil
	//			},
	//		},
	//	},
	//	MutateAndGetPayload: func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
	//		//id, _ := inputMap["id"].(string)
	//		//complete, _ := inputMap["complete"].(bool)
	//		//resolvedId := relay.FromGlobalID(id)
	//		//ChangeTodoStatus(resolvedId.ID, complete)
	//		return map[string]interface{}{
	//			"todoId": "todo-ref-id",
	//		}, nil
	//	},
	//})

	//markAllTodosMutation := relay.MutationWithClientMutationID(relay.MutationConfig{
	//	Name: "MarkAllTodos",
	//	InputFields: graphql.InputObjectConfigFieldMap{
	//		"complete": &graphql.InputObjectFieldConfig{
	//			Type: graphql.NewNonNull(graphql.Boolean),
	//		},
	//	},
	//	OutputFields: graphql.Fields{
	//		"changedTodos": &graphql.Field{
	//			Type: graphql.NewList(todoType),
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				//payload, _ := p.Source.(map[string]interface{})
	//				//todoIds, _ := payload["todoIds"].([]string)
	//				//todos := []*interface{}{}
	//				//for _, todoId := range todoIds {
	//				//	todo := nil
	//				//	if todo != nil {
	//				//		todos = append(todos, todo)
	//				//	}
	//				//}
	//				return nil, nil
	//			},
	//		},
	//		"viewer": &graphql.Field{
	//			Type: userType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				return nil, nil
	//			},
	//		},
	//	},
	//	MutateAndGetPayload: func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
	//		//complete, _ := inputMap["complete"].(bool)
	//		//todoIds := nil
	//		return map[string]interface{}{
	//			"todoIds": "todi-ids",
	//		}, nil
	//	},
	//})

	//removeCompletedTodosMutation := relay.MutationWithClientMutationID(relay.MutationConfig{
	//	Name: "RemoveCompletedTodos",
	//	OutputFields: graphql.Fields{
	//		"deletedTodoIds": &graphql.Field{
	//			Type: graphql.NewList(graphql.String),
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				payload, _ := p.Source.(map[string]interface{})
	//				return payload["todoIds"], nil
	//			},
	//		},
	//		"viewer": &graphql.Field{
	//			Type: userType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				return nil, nil
	//			},
	//		},
	//	},
	//	MutateAndGetPayload: func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
	//		todoIds := []string{}
	//		return map[string]interface{}{
	//			"todoIds": todoIds,
	//		}, nil
	//	},
	//})

	//removeTodoMutation := relay.MutationWithClientMutationID(relay.MutationConfig{
	//	Name: "RemoveTodo",
	//	InputFields: graphql.InputObjectConfigFieldMap{
	//		"id": &graphql.InputObjectFieldConfig{
	//			Type: graphql.NewNonNull(graphql.ID),
	//		},
	//	},
	//	OutputFields: graphql.Fields{
	//		"deletedTodoId": &graphql.Field{
	//			Type: graphql.ID,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				payload, _ := p.Source.(map[string]interface{})
	//				return payload["todoId"], nil
	//			},
	//		},
	//		"viewer": &graphql.Field{
	//			Type: userType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				return nil, nil
	//			},
	//		},
	//	},
	//	MutateAndGetPayload: func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
	//		id, _ := inputMap["id"].(string)
	//		resolvedId := relay.FromGlobalID(id)
	//		//RemoveTodo(resolvedId.ID)
	//		return map[string]interface{}{
	//			"todoId": resolvedId.ID,
	//		}, nil
	//	},
	//})
	//renameTodoMutation := relay.MutationWithClientMutationID(relay.MutationConfig{
	//	Name: "RenameTodo",
	//	InputFields: graphql.InputObjectConfigFieldMap{
	//		"id": &graphql.InputObjectFieldConfig{
	//			Type: graphql.NewNonNull(graphql.ID),
	//		},
	//		"text": &graphql.InputObjectFieldConfig{
	//			Type: graphql.NewNonNull(graphql.String),
	//		},
	//	},
	//	OutputFields: graphql.Fields{
	//		"todo": &graphql.Field{
	//			Type: todoType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				//payload, _ := p.Source.(map[string]interface{})
	//				//todoId, _ := payload["todoId"].(string)
	//				return nil, nil
	//			},
	//		},
	//		"viewer": &graphql.Field{
	//			Type: userType,
	//			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
	//				return nil, nil
	//			},
	//		},
	//	},
	//	MutateAndGetPayload: func(inputMap map[string]interface{}, info graphql.ResolveInfo, ctx context.Context) (map[string]interface{}, error) {
	//		id, _ := inputMap["id"].(string)
	//		resolvedId := relay.FromGlobalID(id)
	//		//text, _ := inputMap["text"].(string)
	//		//RenameTodo(resolvedId.ID, text)
	//		return map[string]interface{}{
	//			"todoId": resolvedId.ID,
	//		}, nil
	//	},
	//})
	mutationType := graphql.NewObject(graphql.ObjectConfig{
		Name:   "Mutation",
		Fields: mutationFields,
	})

	var err error
	Schema, err = graphql.NewSchema(graphql.SchemaConfig{
		Query:    rootQuery,
		Mutation: mutationType,
	})
	if err != nil {
		log.Errorf("Failed to generate graphql schema: %v", err)
	}

	return &Schema

	//for _, table := range cmsConfig.Tables {
	//
	//	for _, relation := range table.Relations {
	//		if relation.Relation == "has_one" || relation.Relation == "belongs_to" {
	//			if relation.Subject == table.TableName {
	//				log.Printf("Add column: %v", table.TableName+"."+relation.GetObjectName())
	//				if done[table.TableName+"."+relation.GetObjectName()] {
	//					continue
	//					panic("ok")
	//				}
	//				done[table.TableName+"."+relation.GetObjectName()] = true
	//				inputTypesMap[table.TableName].Fields()[table.TableName+"."+relation.GetObjectName()] = &graphql.InputObjectField{
	//					Type:        inputTypesMap[relation.GetObject()],
	//					PrivateName: relation.GetObjectName(),
	//				}
	//			} else {
	//				log.Printf("Add column: %v", table.TableName+"."+relation.GetSubjectName())
	//				if done[table.TableName+"."+relation.GetSubjectName()] {
	//					// panic("ok")
	//				}
	//				done[table.TableName+"."+relation.GetSubjectName()] = true
	//				inputTypesMap[table.TableName].Fields()[table.TableName+"."+relation.GetSubjectName()] = &graphql.InputObjectField{
	//					Type:        inputTypesMap[relation.GetSubject()],
	//					PrivateName: relation.GetSubjectName(),
	//				}
	//			}
	//
	//		} else {
	//			if relation.Subject == table.TableName {
	//				log.Printf("Add column: %v", table.TableName+"."+relation.GetObjectName())
	//				if done[table.TableName+"."+relation.GetObjectName()] {
	//					panic("ok")
	//				}
	//				done[table.TableName+"."+relation.GetObjectName()] = true
	//				inputTypesMap[table.TableName].Fields()[table.TableName+"."+relation.GetObjectName()] = &graphql.InputObjectField{
	//					PrivateName: relation.GetObjectName(),
	//					Type:        graphql.NewList(inputTypesMap[relation.GetObject()]),
	//				}
	//			} else {
	//				log.Printf("Add column: %v", table.TableName+"."+relation.GetSubjectName())
	//				if done[table.TableName+"."+relation.GetSubjectName()] {
	//					panic("ok")
	//				}
	//				done[table.TableName+"."+relation.GetSubjectName()] = true
	//				inputTypesMap[table.TableName].Fields()[table.TableName+"."+relation.GetSubjectName()] = &graphql.InputObjectField{
	//					Type:        graphql.NewList(inputTypesMap[relation.GetSubject()]),
	//					PrivateName: relation.GetSubjectName(),
	//				}
	//			}
	//		}
	//	}
	//}

	//for _, table := range cmsConfig.Tables {
	//
	//	createFields := make(graphql.FieldConfigArgument)
	//
	//	for _, column := range table.Columns {
	//
	//		if column.IsForeignKey {
	//			continue
	//		}
	//
	//

	//
	//		if IsStandardColumn(column.ColumnName) {
	//			continue
	//		}
	//
	//		if column.IsForeignKey {
	//			continue
	//		}
	//
	//		if column.IsNullable {
	//			createFields[table.TableName+"."+column.ColumnName] = &graphql.ArgumentConfig{
	//				Type: resource.ColumnManager.GetGraphqlType(column.ColumnType),
	//			}
	//		} else {
	//			createFields[table.TableName+"."+column.ColumnName] = &graphql.ArgumentConfig{
	//				Type: graphql.NewNonNull(resource.ColumnManager.GetGraphqlType(column.ColumnType)),
	//			}
	//		}
	//
	//	}
	//
	//	//for _, relation := range table.Relations {
	//	//
	//	//	if relation.Relation == "has_one" || relation.Relation == "belongs_to" {
	//	//		if relation.Subject == table.TableName {
	//	//			allFields[table.TableName+"."+relation.GetObjectName()] = &graphql.ArgumentConfig{
	//	//				Type: inputTypesMap[relation.GetObject()],
	//	//			}
	//	//		} else {
	//	//			allFields[table.TableName+"."+relation.GetSubjectName()] = &graphql.ArgumentConfig{
	//	//				Type: inputTypesMap[relation.GetSubject()],
	//	//			}
	//	//		}
	//	//
	//	//	} else {
	//	//		if relation.Subject == table.TableName {
	//	//			allFields[table.TableName+"."+relation.GetObjectName()] = &graphql.ArgumentConfig{
	//	//				Type: graphql.NewList(inputTypesMap[relation.GetObject()]),
	//	//			}
	//	//		} else {
	//	//			allFields[table.TableName+"."+relation.GetSubjectName()] = &graphql.ArgumentConfig{
	//	//				Type: graphql.NewList(inputTypesMap[relation.GetSubject()]),
	//	//			}
	//	//		}
	//	//	}
	//	//}
	//
	//	//mutations["create"+Capitalize(table.TableName)] = &graphql.InputObjectFieldConfig{
	//	//	Type:        inputTypesMap[table.TableName],
	//	//	Description: "Create a new " + table.TableName,
	//	//	//Args:        createFields,
	//	//	//Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
	//	//	//	return func(p graphql.ResolveParams) (interface{}, error) {
	//	//	//		log.Printf("create resolve params: %v", p)
	//	//	//
	//	//	//		data := make(map[string]interface{})
	//	//	//
	//	//	//		for key, val := range p.Args {
	//	//	//			data[key] = val
	//	//	//		}
	//	//	//
	//	//	//		model := api2go.NewApi2GoModelWithData(table.TableName, nil, 0, nil, data)
	//	//	//
	//	//	//		pr := &http.Request{
	//	//	//			Method: "PATCH",
	//	//	//		}
	//	//	//		pr = pr.WithContext(p.Context)
	//	//	//		req := api2go.Request{
	//	//	//			PlainRequest: pr,
	//	//	//			QueryParams: map[string][]string{
	//	//	//			},
	//	//	//		}
	//	//	//
	//	//	//		res, err := resources[table.TableName].Create(model, req)
	//	//	//
	//	//	//		return res.Result().(api2go.Api2GoModel).Data, err
	//	//	//	}
	//	//	//}(table),
	//	//}
	//
	//	//mutations["update"+Capitalize(table.TableName)] = &graphql.InputObjectFieldConfig{
	//	//	Type:        inputTypesMap[table.TableName],
	//	//	Description: "Create a new " + table.TableName,
	//	//	//Args:        createFields,
	//	//	//Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
	//	//	//	return func(p graphql.ResolveParams) (interface{}, error) {
	//	//	//		log.Printf("create resolve params: %v", p)
	//	//	//
	//	//	//		data := make(map[string]interface{})
	//	//	//
	//	//	//		for key, val := range p.Args {
	//	//	//			data[key] = val
	//	//	//		}
	//	//	//
	//	//	//		model := api2go.NewApi2GoModelWithData(table.TableName, nil, 0, nil, data)
	//	//	//
	//	//	//		pr := &http.Request{
	//	//	//			Method: "PATCH",
	//	//	//		}
	//	//	//		pr = pr.WithContext(p.Context)
	//	//	//		req := api2go.Request{
	//	//	//			PlainRequest: pr,
	//	//	//			QueryParams: map[string][]string{
	//	//	//			},
	//	//	//		}
	//	//	//
	//	//	//		res, err := resources[table.TableName].Update(model, req)
	//	//	//
	//	//	//		return res.Result().(api2go.Api2GoModel).Data, err
	//	//	//	}
	//	//	//}(table),
	//	//}
	//

	//
	//var rootMutation = graphql.NewObject(graphql.ObjectConfig{
	//	Name:   "RootMutation",
	//	Fields: mutations,
	//});
	//var rootQuery = graphql.NewObject(graphql.ObjectConfig{
	//	Name:   "RootQuery",
	//	Fields: query,
	//})
	//
	//// define schema, with our rootQuery and rootMutation
	//var schema, _ = graphql.NewSchema(graphql.SchemaConfig{
	//	Query:    rootQuery,
	//	Mutation: rootMutation,
	//})
	//
	//return &schema

}
