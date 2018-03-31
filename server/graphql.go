package server

import (
	"log"
	"errors"
	"strings"
	"net/http"
	"encoding/json"
	"encoding/base64"
	"github.com/artpar/api2go"
	"github.com/gedex/inflector"
	"github.com/graphql-go/graphql"
	"github.com/daptin/daptin/server/resource"
)

// Capitalize capitalizes the first character of the string.
func Capitalize(s string) string {
	if len(s) == 1 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[0:1]) + s[1:]
}

func MakeGraphqlSchema(cmsConfig *resource.CmsConfig, resources map[string]*resource.DbResource) *graphql.Schema {

	graphqlTypesMap := make(map[string]*graphql.InputObject)
	mutations := make(graphql.Fields)
	query := make(graphql.Fields)

	for _, table := range cmsConfig.Tables {

		fields := make(graphql.InputObjectConfigFieldMap)

		for _, column := range table.Columns {

			if column.IsForeignKey {
				continue
			}

			fields[column.ColumnName] = &graphql.InputObjectFieldConfig{
				Type: resource.ColumnManager.GetGraphqlType(column.ColumnType),
			}
		}
		fields["__type"] = &graphql.InputObjectFieldConfig{
			Type: resource.ColumnManager.GetGraphqlType("name"),
		}

		objectConfig := graphql.NewInputObject(graphql.InputObjectConfig{
			Name:   table.TableName,
			Fields: fields,
		})

		graphqlTypesMap[table.TableName] = objectConfig

	}

	for _, table := range cmsConfig.Tables {

		for _, relation := range table.Relations {
			if relation.Relation == "has_one" || relation.Relation == "belongs_to" {
				if relation.Subject == table.TableName {
					graphqlTypesMap[table.TableName].Fields()[relation.GetObjectName()] = &graphql.InputObjectField{
						Type:        graphqlTypesMap[relation.GetObject()],
						PrivateName: relation.GetObjectName(),
					}
				} else {
					graphqlTypesMap[table.TableName].Fields()[relation.GetSubjectName()] = &graphql.InputObjectField{
						Type:        graphqlTypesMap[relation.GetSubject()],
						PrivateName: relation.GetSubjectName(),
					}
				}

			} else {
				if relation.Subject == table.TableName {
					graphqlTypesMap[table.TableName].Fields()[relation.GetObjectName()] = &graphql.InputObjectField{
						PrivateName: relation.GetObjectName(),
						Type:        graphql.NewList(graphqlTypesMap[relation.GetObject()]),
					}
				} else {
					graphqlTypesMap[table.TableName].Fields()[relation.GetSubjectName()] = &graphql.InputObjectField{
						Type:        graphql.NewList(graphqlTypesMap[relation.GetSubject()]),
						PrivateName: relation.GetSubjectName(),
					}
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

		mutations["create"+Capitalize(table.TableName)] = &graphql.Field{
			Type:        graphqlTypesMap[table.TableName],
			Description: "Create a new " + table.TableName,
			Args:        createFields,
			Resolve: func(table resource.TableInfo) (func(params graphql.ResolveParams) (interface{}, error)) {
				return func(p graphql.ResolveParams) (interface{}, error) {
					log.Printf("create resolve params: %v", p)

					data := make(map[string]interface{})

					for key, val := range p.Args {
						data[key] = val
					}

					model := api2go.NewApi2GoModelWithData(table.TableName, nil, 0, nil, data)

					pr := &http.Request{
						Method: "PATCH",
					}
					pr = pr.WithContext(p.Context)
					req := api2go.Request{
						PlainRequest: pr,
						QueryParams: map[string][]string{
						},
					}

					res, err := resources[table.TableName].Create(model, req)

					return res.Result().(api2go.Api2GoModel).Data, err
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

					data := make(map[string]interface{})

					for key, val := range p.Args {
						data[key] = val
					}

					model := api2go.NewApi2GoModelWithData(table.TableName, nil, 0, nil, data)

					pr := &http.Request{
						Method: "PATCH",
					}
					pr = pr.WithContext(p.Context)
					req := api2go.Request{
						PlainRequest: pr,
						QueryParams: map[string][]string{
						},
					}

					res, err := resources[table.TableName].Update(model, req)

					return res.Result().(api2go.Api2GoModel).Data, err
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

					pr := &http.Request{
						Method: "GET",
					}
					pr = pr.WithContext(params.Context)
					jsStr, err := json.Marshal(filters)
					req := api2go.Request{
						PlainRequest: pr,
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

					pr := &http.Request{
						Method: "GET",
					}
					pr = pr.WithContext(params.Context)
					jsStr, err := json.Marshal(filters)
					req := api2go.Request{
						PlainRequest: pr,
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

		//query["meta"+Capitalize(inflector.Pluralize(table.TableName))] = &graphql.Field{
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
