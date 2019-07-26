package apiblueprint

import (
	"bytes"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"

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
		"type":     typ,
		"required": colInfo.IsNullable,
	}
	return m
}

func BuildApiBlueprint(config *resource.CmsConfig, cruds map[string]*resource.DbResource) string {

	tableMap := map[string]resource.TableInfo{}
	for _, table := range config.Tables {
		tableMap[table.TableName] = table
	}

	apiDefinition := make(map[string]interface{})

	apiDefinition["title"] = "Daptin server"
	apiDefinition["version"] = "v1"
	apiDefinition["baseUri"] = fmt.Sprintf("http://%v", config.Hostname)
	apiDefinition["mediaType"] = "application/json"
	apiDefinition["protocols"] = []string{"HTTP", "HTTPS"}

	typeMap := make(map[string]map[string]interface{})

	relatedStructureType := make(map[string]interface{})
	relatedStructureType["type"] = "object"
	relatedStructureType["properties"] = map[string]interface{}{
		"id": map[string]interface{}{
			"type":        "string",
			"description": "Id of the object",
		},
		"type": map[string]interface{}{
			"type":        "string",
			"description": "Type of the included object",
		},
	}
	typeMap["RelatedStructure"] = relatedStructureType

	paginationObject := make(map[string]interface{})
	paginationObject["type"] = "object"
	paginationObject["properties"] = map[string]interface{}{
		"page[number]": map[string]interface{}{
			"type":        "number",
			"description": "Id of the included object",
		},
		"page[size]": map[string]interface{}{
			"type":        "number",
			"description": "Type of the included object",
		},
	}
	typeMap["Pagination"] = paginationObject

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

	IncludedRelationship := make(map[string]interface{})
	IncludedRelationship["type"] = "object"
	IncludedRelationship["properties"] = map[string]interface{}{
		"data": map[string]interface{}{
			"type":        "RelatedStructure",
			"description": "Associated objects which are also included in the current response",
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

	worldActionMap := make(map[string][]resource.Action)

	for _, tableInfo := range config.Tables {
		ramlType := make(map[string]interface{})
		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		ramlType["type"] = "object"
		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		ramlType["properties"] = properties
		typeMap[tableInfo.TableName] = ramlType

		worldActions, err := cruds["action"].GetActionsByType(tableInfo.TableName)
		if InfoError(err, "Failed to list world actions for raml") {
			continue
		}

		worldActionMap[tableInfo.TableName] = worldActions
		for _, action := range worldActions {
			ramlActionType := make(map[string]interface{})
			ramlActionType["type"] = "object"

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
			typeMap[fmt.Sprintf("%sObject", TitleCase(action.Name))] = ramlActionType

		}

	}

	apiDefinition["types"] = typeMap

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

		resourceInstance["displayName"] = ProperCase(tableInfo.TableName)
		resourceInstance["description"] = "Resources in this group are related to " + tableInfo.TableName

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

		nestedMap := make(map[string]interface{})

		byIdResource := CreateByIdResource(tableInfo)

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

		nestedMap["/{referenceId}"] = byIdResource

		for _, rel := range tableInfo.Relations {

			// BEGIN: Get Relations Method

			relationsById := CreateRelationsByIdResource(tableInfo)

			if tableInfo.TableName == rel.Subject {
				relatedTable := tableInfoMap[rel.Object]
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				relationsById["get"] = getMethod
				relationsById["delete"] = deleteMethod
				nestedMap[fmt.Sprintf("/{referenceId}/%s", rel.Object)] = relationsById
			} else {
				relatedTable := tableInfoMap[rel.Subject]
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				relationsById["get"] = getMethod
				relationsById["delete"] = deleteMethod
				nestedMap[fmt.Sprintf("/{referenceId}/%s", rel.Subject)] = relationsById
			}
			// END: Get relations method

		}

		for k, v := range nestedMap {
			resourceInstance[k] = v
		}

		if tableInfo.IsStateTrackingEnabled {

			//tableInfo.StateMachines

		}

		resourcesMap["/api/"+tableInfo.TableName] = resourceInstance
	}

	actionResource := make(map[string]interface{})

	for _, tableInfo := range config.Tables {

		tableActionResource := make(map[string]interface{})
		tableActionResource["displayName"] = fmt.Sprintf("Actions defined over %s", ProperCase(tableInfo.TableName))
		tableActionResource["description"] = fmt.Sprintf("Actions defined over %s", ProperCase(tableInfo.TableName))

		worldActions, ok := worldActionMap[tableInfo.TableName]

		if len(worldActions) == 0 {
			continue
		}

		if ok {
			for _, action := range worldActions {

				actionResource := CreateActionResource(action)
				actionResource["displayName"] = action.Name
				actionResource["description"] = action.Name
				actionResource["post"] = CreateActionPostMethod(action)
				tableActionResource[fmt.Sprintf("/%s", action.Name)] = actionResource
			}
		} else {
			continue
		}
		actionResource["/action/"+tableInfo.TableName] = tableActionResource

	}

	for key, val := range resourcesMap {
		apiDefinition[key] = val
	}
	for n, v := range actionResource {
		apiDefinition[n] = v
	}

	ym, _ := yaml.Marshal(apiDefinition)
	return "#%RAML 1.0\n" + string(ym)

}

func CreateActionPostMethod(action resource.Action) map[string]interface{} {

	dataInResponse := CreateActionResponse(action)
	postMethod := make(map[string]interface{})
	postMethod["displayName"] = action.Name
	postMethod["description"] = action.Name
	postBody := make(map[string]interface{})

	postBody["type"] = TitleCase(action.Name) + "Object"

	postMethod["body"] = postBody
	postResponseMap := make(map[string]interface{})
	postOkResponse := make(map[string]interface{})
	postResponseBody := make(map[string]interface{})
	postResponseBody["type"] = "object"

	postResponseBody = map[string]interface{}{
		"type":       "object",
		"properties": dataInResponse,
	}
	postOkResponse["body"] = map[string]interface{}{
		"application/vnd.api+json": postResponseBody,
	}

	postResponseMap["200"] = postOkResponse
	postMethod["responses"] = postResponseMap
	return postMethod

}
func CreateActionResponse(action resource.Action) map[string]interface{} {
	resp := make(map[string]interface{})

	for _, outcome := range action.OutFields {

		if outcome.SkipInResponse {
			continue
		}

		attrs := CreateActionResponseTypeAttributes(outcome)

		for key := range attrs {
			resp[key] = "string"
		}
	}

	return resp
}
func CreateActionResponseTypeAttributes(outcome resource.Outcome) map[string]interface{} {
	properties := map[string]interface{}{}

	for attrName := range outcome.Attributes {
		properties[attrName] = "string"
	}

	return properties

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
				"type": tableInfo.TableName,
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
	postMethod["displayName"] = fmt.Sprintf("Create a new %s", tableInfo.TableName)
	postMethod["description"] = fmt.Sprintf("Create a new %v", tableInfo.TableName)
	postBody := make(map[string]interface{})
	postBody["type"] = tableInfo.TableName
	postMethod["body"] = postBody
	postResponseMap := make(map[string]interface{})
	postOkResponse := make(map[string]interface{})
	postResponseBody := make(map[string]interface{})
	postResponseBody["type"] = "object"

	postResponseBody = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"data": dataInResponse,
			"links": map[string]interface{}{
				"type": "PaginationStatus",
			},
		},
	}
	postOkResponse["body"] = map[string]interface{}{
		"application/vnd.api+json": postResponseBody,
	}

	postResponseMap["200"] = postOkResponse
	postMethod["responses"] = postResponseMap
	return postMethod
}
func CreateGetAllMethod(tableInfo resource.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getAllMethod := make(map[string]interface{})
	getAllMethod["description"] = fmt.Sprintf("Returns a list of %v", ProperCase(tableInfo.TableName))
	getAllMethod["displayName"] = fmt.Sprintf("Get " + tableInfo.TableName)
	getAllMethod["queryParameters"] = map[string]map[string]interface{}{
		"sort": {
			"type":        "string",
			"required":    false,
			"description": "Field name to sort by",
		},
		"page[number]": {
			"type":        "string",
			"required":    false,
			"description": "Page number for the query set, starts with 1",
		},
		"page[size]": {
			"type":        "string",
			"required":    false,
			"description": "Size of one page, try 10",
		},
		"query": {
			"type":        "string",
			"required":    false,
			"description": "search text in indexed columns",
		},
	}
	getResponseMap := make(map[string]interface{})
	get200Response := make(map[string]interface{})
	get200Response["body"] = map[string]interface{}{
		"application/vnd.api+json": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": dataInResponse,
				"links": map[string]interface{}{
					"type": "PaginationStatus",
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

func TitleCase(str string) string {
	st := str[1:]
	st = strings.Replace(st, "_", "", -1)
	st = strings.Replace(st, ".", "", -1)

	return strings.ToUpper(str[0:1]) + st
}

func CreateDeleteMethod(tableInfo resource.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdMethod200Response["body"] = nil
	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdResponseMap["200"] = deleteByIdMethod200Response
	deleteByIdMethod["responses"] = deleteByIdResponseMap
	deleteByIdMethod["description"] = fmt.Sprintf("Delete a %v", tableInfo.TableName)
	return deleteByIdMethod
}

func CreateDeleteRelationMethod(tableInfo resource.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdMethod200Response["body"] = nil
	deleteBody := make(map[string]interface{})
	deleteBody["type"] = tableInfo.TableName
	deleteByIdMethod["body"] = map[string]interface{}{}

	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdResponseMap["200"] = deleteByIdMethod200Response
	deleteByIdMethod["responses"] = deleteByIdResponseMap
	deleteByIdMethod["description"] = fmt.Sprintf("Remove a related %v from the parent object", tableInfo.TableName)
	return deleteByIdMethod
}

func CreateByIdResource(tableInfo resource.TableInfo) map[string]interface{} {
	byIdResource := make(map[string]interface{})
	byIdResource["uriParameters"] = map[string]interface{}{
		"referenceId": map[string]interface{}{
			"type":        "string",
			"description": "Reference id of the " + tableInfo.TableName + " to be fetched",
			"required":    true,
		},
	}
	return byIdResource
}

func CreateActionResource(action resource.Action) map[string]interface{} {
	byIdResource := make(map[string]interface{})
	byIdResource["description"] = fmt.Sprintf("Action %s", action.Name)
	return byIdResource
}

func CreateRelationsByIdResource(tableInfo resource.TableInfo) map[string]interface{} {
	byIdResource := make(map[string]interface{})
	byIdResource["uriParameters"] = map[string]interface{}{
		"referenceId": map[string]interface{}{
			"type":        "string",
			"description": "Reference id of the " + tableInfo.TableName + " to be fetched",
			"required":    true,
		},
	}
	return byIdResource
}

func CreateGetMethod(tableInfo resource.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getByIdMethod := make(map[string]interface{})
	getByIdMethod200Response := make(map[string]interface{})
	getByIdMethod200Response["body"] = map[string]interface{}{
		"application/vnd.api+json": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": dataInResponse,
				"links": map[string]interface{}{
					"type": "PaginationStatus",
				},
			},
		},
	}

	getByIdResponseMap := make(map[string]interface{})
	getByIdResponseMap["200"] = getByIdMethod200Response
	getByIdMethod["responses"] = getByIdResponseMap
	getByIdMethod["description"] = fmt.Sprintf("Get %v by id", tableInfo.TableName)
	return getByIdMethod
}
func CreatePatchMethod(tableInfo resource.TableInfo) map[string]interface{} {

	patchMethod := make(map[string]interface{})
	patchMethod["displayName"] = fmt.Sprintf("Edit an existing %s", tableInfo.TableName)
	patchMethod["description"] = fmt.Sprintf("Edit an existing %s", tableInfo.TableName)
	patchBody := make(map[string]interface{})
	patchBody["type"] = tableInfo.TableName
	patchMethod["body"] = patchBody
	patchResponseMap := make(map[string]interface{})
	patchOkResponse := make(map[string]interface{})
	patchResponseBody := make(map[string]interface{})
	patchResponseBody["type"] = "object"
	patchRelationshipMap := make(map[string]interface{}, 0)
	for _, relation := range tableInfo.Relations {
		if relation.Object == tableInfo.TableName {
			patchRelationshipMap[relation.SubjectName] = "IncludedRelationship"
		} else {
			patchRelationshipMap[relation.ObjectName] = "IncludedRelationship"
		}
	}
	var patchDataInResponse = map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"attributes": map[string]interface{}{
				"type": tableInfo.TableName,
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
				"type": "PaginationStatus",
			},
		},
	}
	patchOkResponse["body"] = map[string]interface{}{
		"application/vnd.api+json": patchResponseBody,
	}
	patchResponseMap["200"] = patchOkResponse
	patchMethod["responses"] = patchResponseMap
	return patchMethod
}

//func CreateForwardRelationLine(relation api2go.TableRelation) map[string]interface{} {
//
//	relationDescription := relation.GetRelation()
//
//	otherObjectName := relation.GetObject()
//	switch relationDescription {
//	case "has_one":
//		relationDescription = "Has one " + otherObjectName
//	case "has_many":
//		relationDescription = "Has many " + otherObjectName
//	case "belongs_to":
//		relationDescription = "Belongs to " + otherObjectName
//	case "has_many_and_belongs_to_many":
//		relationDescription = "Has many and belongs to " + otherObjectName
//	}
//
//	return fmt.Sprintf("      %s: %s", relation.GetObjectName(), otherObjectName)
//}

//func CreateBackwardRelationLine(relation api2go.TableRelation) string {
//	relationDescription := relation.GetRelation()
//
//	otherObjectName := relation.GetSubject()
//	switch relationDescription {
//	case "has_one":
//		relationDescription = "Has one " + otherObjectName
//	case "has_many":
//		relationDescription = "Has many " + otherObjectName
//	case "belongs_to":
//		relationDescription = "Belongs to " + otherObjectName
//	case "has_many_and_belongs_to_many":
//		relationDescription = "Has many and belongs to " + otherObjectName
//	}
//
//	return fmt.Sprintf("      %s: %s", relation.GetSubjectName(), otherObjectName)
//}
