package apiblueprint

import (
	"bytes"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"
	"github.com/iancoleman/strcase"

	"fmt"
	"strings"
	//"github.com/daptin/daptin/server/fakerservice"
	"github.com/advance512/yaml"
	log "github.com/sirupsen/logrus"
)

func InfoError(err error, args ...interface{}) bool {
	if err != nil {
		if len(args) > 0 {
			fm := args[0].(string) + ": %v"
			args = args[1:]
			args = append(args, err)
			log.Printf(fm, args...)
			return true
		} else {
			log.Printf("%v", err)
			return true
		}
	}
	return false
}

type BlueprintWriter struct {
	buffer bytes.Buffer
}

func NewBluePrintWriter() BlueprintWriter {
	var x = BlueprintWriter{}

	x.buffer = bytes.Buffer{}
	return x
}

func (x *BlueprintWriter) WriteString(s string) {
	x.buffer.WriteString(s + "\n")
}

func (x *BlueprintWriter) WriteStringf(s ...interface{}) {
	x.buffer.WriteString(fmt.Sprintf(s[0].(string)+"\n", s[1:]...))
}

func (x *BlueprintWriter) Markdown() string {
	return x.buffer.String()
}

var skipColumns = map[string]bool{
	"id":         true,
	"permission": true,
	"status":     true,
}

func CreateColumnLine(colInfo api2go.ColumnInfo) map[string]interface{} {
	columnType := colInfo.ColumnType
	typ := resource.ColumnManager.GetBlueprintType(columnType)

	if typ == "" {
		typ = "string"
	}

	m := map[string]interface{}{
		"type": typ,
	}
	return m
}

func BuildApiBlueprint(config *resource.CmsConfig, cruds map[string]*resource.DbResource) string {

	tableMap := map[string]resource.TableInfo{}
	for _, table := range config.Tables {
		tableMap[table.TableName] = table
	}

	apiDefinition := make(map[string]interface{})

	apiDefinition["openapi"] = "3.0.0"
	apiDefinition["info"] = map[string]interface{}{
		"version": "1.0.0",
		"title":   "Daptin API endpoint",
		"license": map[string]interface{}{
			"name": "MIT",
		},
		"contact": map[string]interface{}{
			"name": "Parth",
		},
		"description":    "Daptin server API spec",
		"termsOfService": config.Hostname + "/tos",
	}

	apiDefinition["servers"] = []map[string]interface{}{
		{
			"url":         fmt.Sprintf("http://%v", config.Hostname),
			"description": "Server " + config.Hostname,
		},
	}
	typeMap := make(map[string]map[string]interface{})
	typeMap["RelatedStructure"] = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"id": map[string]interface{}{
				"type":        "string",
				"description": "Id of the object",
			},
			"type": map[string]interface{}{
				"type":        "string",
				"description": "Type of the included object",
			},
		},
	}

	paginationObject := make(map[string]interface{})
	paginationObject["type"] = "object"
	paginationObject["properties"] = map[string]interface{}{
		"page[number]": map[string]interface{}{
			"type":        "number",
			"description": "Page number",
		},
		"page[size]": map[string]interface{}{
			"type":        "number",
			"description": "Number of item to return",
		},
		"page[after]": map[string]interface{}{
			"type":        "string",
			"description": "Reference id of the object after which to look for",
		},
	}
	typeMap["Pagination"] = paginationObject

	actionResponse := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"ResponseType": map[string]interface{}{
				"type": "string",
			},
			"Attributes": map[string]interface{}{
				"type": "object",
			},
		},
	}

	paginationStatus := make(map[string]interface{})
	paginationStatus["type"] = "object"
	paginationStatus["properties"] = map[string]interface{}{
		"current_page": map[string]interface{}{
			"type":        "number",
			"description": "The current page, for pagination",
		},
		"from": map[string]interface{}{
			"type":        "number",
			"description": "From page",
		},
		"last_page": map[string]interface{}{
			"type":        "number",
			"description": "The last page number in current query set",
		},
		"per_page": map[string]interface{}{
			"type":        "number",
			"description": "This is the number of results in one page",
		},
		"to": map[string]interface{}{
			"type":        "number",
			"description": "Index of the last record feched in this result",
		},
		"total": map[string]interface{}{
			"type":        "number",
			"description": "Total number of records",
		},
	}
	typeMap["PaginationStatus"] = paginationStatus
	typeMap["ActionResponse"] = actionResponse

	IncludedRelationship := make(map[string]interface{})
	IncludedRelationship["type"] = "object"
	IncludedRelationship["properties"] = map[string]interface{}{
		"data": map[string]interface{}{
			"$ref": "#/components/schemas/RelatedStructure",
		},
		"links": map[string]interface{}{
			"type":        "object",
			"description": "From page",
			"properties": map[string]interface{}{
				"related": map[string]interface{}{
					"type":        "string",
					"description": "link to related objects",
				},
				"self": map[string]interface{}{
					"type":        "string",
					"description": "link to self",
				},
			},
		},
	}
	typeMap["IncludedRelationship"] = IncludedRelationship

	for _, tableInfo := range config.Tables {
		ramlType := make(map[string]interface{})
		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		requiredCols := make([]string, 0)
		ramlType["type"] = "object"
		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			if !colInfo.IsNullable && !resource.IsStandardColumn(colInfo.ColumnName) {
				requiredCols = append(requiredCols, colInfo.ColumnName)
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		ramlType["properties"] = properties
		ramlType["required"] = requiredCols

		typeMap[strcase.ToCamel(tableInfo.TableName)] = ramlType

		//worldActions, err := cruds["action"].GetActionsByType(tableInfo.TableName)
		//if InfoError(err, "Failed to list world actions for raml") {
		//	continue
		//}

	}
	for _, tableInfo := range config.Tables {
		ramlType := make(map[string]interface{})
		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		requiredCols := make([]string, 0)
		ramlType["type"] = "object"
		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}
			if resource.IsStandardColumn(colInfo.ColumnName) {
				continue
			}

			if !colInfo.IsNullable && colInfo.DefaultValue == "" {
				requiredCols = append(requiredCols, colInfo.ColumnName)
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		ramlType["properties"] = properties
		ramlType["required"] = requiredCols

		typeMap["New"+strcase.ToCamel(tableInfo.TableName)] = ramlType

	}

	for _, action := range config.Actions {
		ramlActionType := make(map[string]interface{})
		// ramlActionType["type"] = "object"

		actionProperties := make(map[string]interface{})
		for _, colInfo := range action.InFields {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			actionProperties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}
		ramlActionType["properties"] = actionProperties
		typeMap[fmt.Sprintf("%s%sObject", strcase.ToCamel(action.Name), strcase.ToCamel(action.OnType))] = ramlActionType

	}

	apiDefinition["components"] = map[string]interface{}{
		"schemas": typeMap,
	}

	resourcesMap := map[string]map[string]interface{}{}
	tableInfoMap := make(map[string]resource.TableInfo)
	for _, tableInfo := range config.Tables {
		tableInfoMap[tableInfo.TableName] = tableInfo
	}

	for _, tableInfo := range config.Tables {

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		resourceInstance := make(map[string]interface{})

		dataInResponse := CreateDataInResponse(tableInfo)

		// BEGIN: POST request
		postMethod := CreatePostMethod(tableInfo, dataInResponse)
		resourceInstance["post"] = &postMethod
		//  END: POST Request

		//  BEGIN: GET Request
		getAllMethod := CreateGetAllMethod(tableInfo, dataInResponse)
		resourceInstance["get"] = &getAllMethod
		//  END: GET Request

		//fakeObject := fakerservice.NewFakeInstance(tableInfo)

		nestedMap := make(map[string]map[string]interface{})

		byIdResource := make(map[string]interface{})

		//  BEGIN: GET ById Request
		getByIdMethod := CreateGetMethod(tableInfo, dataInResponse)
		byIdResource["get"] = getByIdMethod
		//  END: GET ById Request

		// BEGIN: PATCH request
		patchMethod := CreatePatchMethod(tableInfo)
		byIdResource["patch"] = &patchMethod
		//  END: PATCH Request

		// BEGIN: DELETE Request
		deleteByIdMethod := CreateDeleteMethod(tableInfo)
		byIdResource["delete"] = deleteByIdMethod
		// END: DELETE Request

		nestedMap["/api/"+tableInfo.TableName+"/{referenceId}"] = byIdResource

		for _, rel := range tableInfo.Relations {

			// BEGIN: Get Relations Method

			relationsById := make(map[string]interface{})

			if tableInfo.TableName == rel.Subject {
				relatedTable := tableInfoMap[rel.Object]
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))

				getMethod["operationId"] = "Related" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				getMethod["summary"] = fmt.Sprintf("Fetch related %s of %v", rel.ObjectName, tableInfo.TableName)

				getMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				deleteMethod["summary"] = fmt.Sprintf("Delete related %s of %v", rel.ObjectName, tableInfo.TableName)
				deleteMethod["operationId"] = "Delete" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)

				relationsById["get"] = getMethod
				relationsById["delete"] = deleteMethod

				deleteMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				nestedMap[fmt.Sprintf("/api/%s/{referenceId}/%s", tableInfo.TableName, rel.Object)] = relationsById
			} else {
				relatedTable := tableInfoMap[rel.Subject]
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				getMethod["operationId"] = "Related" + strcase.ToCamel(rel.SubjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				patchMethod["summary"] = fmt.Sprintf("Fetch related %s of %v", rel.SubjectName, tableInfo.TableName)
				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				deleteMethod["operationId"] = "Delete" + strcase.ToCamel(relatedTable.TableName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				patchMethod["summary"] = fmt.Sprintf("Delete related %s of %v", rel.SubjectName, tableInfo.TableName)
				relationsById["get"] = getMethod

				getMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				deleteMethod["parameters"] = []map[string]interface{}{
					{
						"name": "referenceId",
						"schema": map[string]interface{}{
							"type": "string",
						},
						"required":    true,
						"in":          "path",
						"description": "Reference Id of the " + tableInfo.TableName,
					},
				}

				relationsById["delete"] = deleteMethod
				nestedMap[fmt.Sprintf("/api/%s/{referenceId}/%s", tableInfo.TableName, rel.Subject)] = relationsById
			}
			// END: Get relations method

		}

		for k, v := range nestedMap {
			resourcesMap[k] = v
		}

		if tableInfo.IsStateTrackingEnabled {

			//tableInfo.StateMachines

		}

		resourcesMap["/api/"+tableInfo.TableName] = resourceInstance
	}

	for _, action := range config.Actions {

		resourcesMap[fmt.Sprintf("/action/%s/%s", action.OnType, action.Name)] = map[string]interface{}{
			"post": map[string]interface{}{
				"tags":        []string{action.OnType},
				"operationId": "Execute" + strcase.ToCamel(action.Name) + "On" + strcase.ToCamel(action.OnType),
				"summary":     action.Label,
				"requestBody": map[string]interface{}{
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/" + fmt.Sprintf("%s%sObject", strcase.ToCamel(action.Name), strcase.ToCamel(action.OnType)),
							},
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{

						"description": "action response of " + action.Name,
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"items": map[string]interface{}{
										"$ref": "#/components/schemas/ActionResponse",
									},
								},
							},
						},
					},
				},
			},
		}

	}

	actionResource := make(map[string]interface{})

	apiDefinition["paths"] = resourcesMap
	for n, v := range actionResource {
		apiDefinition[n] = v
	}

	ym, _ := yaml.Marshal(apiDefinition)
	return string(ym)

}

func CreateDataInResponse(tableInfo resource.TableInfo) map[string]interface{} {
	relationshipMap := make(map[string]interface{}, 0)
	for _, relation := range tableInfo.Relations {
		if relation.Object == tableInfo.TableName {
			relationshipMap[relation.SubjectName] = "IncludedRelationship"
		} else {
			relationshipMap[relation.ObjectName] = "IncludedRelationship"
		}
	}

	var dataInResponse = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"attributes": map[string]interface{}{
				"schema": map[string]interface{}{
					"$ref": "#/components/schemas/New" + strcase.ToCamel(tableInfo.TableName),
				},
			},
			"id": map[string]interface{}{
				"type": "string",
			},
			"type": map[string]interface{}{
				"type": "string",
			},
			"relationships": map[string]interface{}{
				"type":       "object",
				"properties": relationshipMap,
			},
		},
	}
	return dataInResponse
}
func CreatePostMethod(tableInfo resource.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	postMethod := make(map[string]interface{})
	postMethod["operationId"] = fmt.Sprintf("Create%s", strcase.ToCamel(tableInfo.TableName))
	postMethod["summary"] = fmt.Sprintf("Create a new %v", tableInfo.TableName)
	postMethod["tags"] = []string{tableInfo.TableName}
	postBody := make(map[string]interface{})

	postBody["description"] = tableInfo.TableName + " to create"
	postBody["required"] = true
	postBody["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"attributes": map[string]interface{}{
								"$ref": "#/components/schemas/New" + strcase.ToCamel(tableInfo.TableName),
							},
						},
					},
				},
			},
		},
	}
	postMethod["requestBody"] = postBody
	postResponseMap := make(map[string]interface{})
	postResponseBody := make(map[string]interface{})
	postResponseBody["type"] = "object"

	postResponseBody = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": map[string]interface{}{
				"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
			},
			"links": map[string]interface{}{
				"$ref": "#/components/schemas/PaginationStatus",
			},
		},
	}
	postOkResponse := make(map[string]interface{})
	postOkResponse["description"] = tableInfo.TableName + " response"

	postOkResponse["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": postResponseBody,
		},
	}

	postResponseMap["200"] = postOkResponse
	postMethod["responses"] = postResponseMap
	return postMethod
}
func CreateGetAllMethod(tableInfo resource.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getAllMethod := make(map[string]interface{})
	getAllMethod["description"] = fmt.Sprintf("Returns a list of %v", ProperCase(tableInfo.TableName))
	getAllMethod["operationId"] = fmt.Sprintf("Get" + tableInfo.TableName)
	getAllMethod["summary"] = fmt.Sprintf("List all %v", tableInfo.TableName)
	getAllMethod["tags"] = []string{tableInfo.TableName}
	getAllMethod["parameters"] = []map[string]interface{}{
		{
			"name": "sort",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    false,
			"in":          "query",
			"description": "Field name to sort by",
		},
		{
			"name": "page[number]",
			"in":   "query",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    false,
			"description": "Page number for the query set, starts with 1",
		},
		{
			"schema": map[string]interface{}{
				"type": "string",
			},
			"name":        "page[size]",
			"required":    false,
			"in":          "query",
			"description": "Size of one page, try 10",
		},
		{
			"schema": map[string]interface{}{
				"type": "string",
			},
			"in":          "query",
			"name":        "query",
			"required":    false,
			"description": "search text in indexed columns",
		},
	}
	getResponseMap := make(map[string]interface{})
	get200Response := make(map[string]interface{})
	get200Response["description"] = "list of all " + tableInfo.TableName
	get200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
				},
			},
		},
	}

	getResponseMap["200"] = get200Response
	getAllMethod["responses"] = getResponseMap
	return getAllMethod
}
func ProperCase(str string) string {
	st := str[1:]
	st = strings.Replace(st, "_", " ", -1)
	st = strings.Replace(st, ".", " ", -1)
	return strings.ToUpper(str[0:1]) + st
}

func CreateDeleteMethod(tableInfo resource.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdMethod200Response["description"] = "delete " + tableInfo.TableName + " by reference id"
	deleteByIdResponseMap["200"] = deleteByIdMethod200Response
	deleteByIdMethod["responses"] = deleteByIdResponseMap

	deleteByIdMethod["description"] = fmt.Sprintf("Delete a %v", tableInfo.TableName)

	deleteByIdMethod["summary"] = fmt.Sprintf("Delete %v", tableInfo.TableName)
	deleteByIdMethod["tags"] = []string{tableInfo.TableName}
	deleteByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}

	return deleteByIdMethod
}

func CreateDeleteRelationMethod(tableInfo resource.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdMethod200Response["description"] = "Successful deletion of " + tableInfo.TableName
	deleteBody := make(map[string]interface{})
	deleteBody["type"] = tableInfo.TableName
	deleteByIdMethod["description"] = "Delete a " + tableInfo.TableName

	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdResponseMap["200"] = deleteByIdMethod200Response
	deleteByIdMethod["responses"] = deleteByIdResponseMap
	deleteByIdMethod["description"] = fmt.Sprintf("Remove a related %v from the parent object", tableInfo.TableName)
	deleteByIdMethod["tags"] = []string{tableInfo.TableName}
	deleteByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}

	return deleteByIdMethod
}

func CreateGetMethod(tableInfo resource.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getByIdMethod := make(map[string]interface{})
	getByIdMethod200Response := make(map[string]interface{})
	getByIdMethod["tags"] = []string{tableInfo.TableName}
	getByIdMethod200Response["description"] = "get " + tableInfo.TableName + " by reference id"
	getByIdMethod200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
			},
		},
	}

	getByIdResponseMap := make(map[string]interface{})
	getByIdResponseMap["200"] = getByIdMethod200Response
	getByIdMethod["responses"] = getByIdResponseMap

	getByIdMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}

	getByIdMethod["summary"] = fmt.Sprintf("Get %v by id", tableInfo.TableName)
	getByIdMethod["description"] = fmt.Sprintf("Get %v by id", tableInfo.TableName)
	return getByIdMethod
}
func CreatePatchMethod(tableInfo resource.TableInfo) map[string]interface{} {

	patchMethod := make(map[string]interface{})
	patchMethod["operationId"] = fmt.Sprintf("Update%s", strcase.ToCamel(tableInfo.TableName))
	patchMethod["summary"] = fmt.Sprintf("Update existing %v", tableInfo.TableName)
	patchMethod["description"] = fmt.Sprintf("Edit an existing %s", tableInfo.TableName)
	patchMethod["tags"] = []string{tableInfo.TableName}
	patchBody := make(map[string]interface{})
	patchBody["type"] = tableInfo.TableName
	patchResponseMap := make(map[string]interface{})
	patchOkResponse := make(map[string]interface{})
	patchResponseBody := make(map[string]interface{})
	patchResponseBody["type"] = "object"
	patchRelationshipMap := make(map[string]interface{}, 0)

	patchMethod["requestBody"] = map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"attributes": map[string]interface{}{
									"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
								},
								"id": map[string]interface{}{
									"type": "string",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, relation := range tableInfo.Relations {
		if relation.Object == tableInfo.TableName {
			patchRelationshipMap[relation.SubjectName] = map[string]interface{}{
				"$ref": "#/components/schemas/IncludedRelationship",
			}
		} else {
			patchRelationshipMap[relation.ObjectName] = map[string]interface{}{
				"$ref": "#/components/schemas/IncludedRelationship",
			}
		}
	}
	var patchDataInResponse = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"attributes": map[string]interface{}{
				"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
			},
			"id": map[string]interface{}{
				"type": "string",
			},
			"type": map[string]interface{}{
				"type": "string",
			},
			"relationships": map[string]interface{}{
				"type":       "object",
				"properties": patchRelationshipMap,
			},
		},
	}
	patchResponseBody = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": patchDataInResponse,
			"links": map[string]interface{}{
				"$ref": "#/components/schemas/PaginationStatus",
			},
		},
	}
	patchOkResponse["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": patchResponseBody,
		},
	}
	patchOkResponse["description"] = "updated " + tableInfo.TableName
	patchResponseMap["200"] = patchOkResponse
	patchMethod["parameters"] = []map[string]interface{}{
		{
			"name": "referenceId",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    true,
			"in":          "path",
			"description": "Reference Id of the " + tableInfo.TableName,
		},
	}
	patchMethod["responses"] = patchResponseMap
	return patchMethod
}
