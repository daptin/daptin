package apiblueprint

import (
	"bytes"
	"github.com/artpar/api2go"
	"github.com/daptin/daptin/server/resource"

	"fmt"
	"strings"
	//"github.com/daptin/daptin/server/fakerservice"
	"github.com/advance512/yaml"
)

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
			"description": "Id of the included object",
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

	for _, tableInfo := range config.Tables {
		ramlType := make(map[string]interface{})

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		ramlType["type"] = "object"
		//blueprintWriter.WriteString("  " + tableInfo.TableName + ":")
		//blueprintWriter.WriteString("    type: object")
		//blueprintWriter.WriteString("    properties:")

		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		//for _, relation := range tableInfo.Relations {
		//	if relation.Subject == tableInfo.TableName {
		//		properties[relation.GetObjectName()] = relation.GetObject()
		//	} else {
		//		properties[relation.GetSubjectName()] = relation.GetSubject()
		//	}
		//}

		ramlType["properties"] = properties
		typeMap[tableInfo.TableName] = ramlType
	}

	apiDefinition["types"] = typeMap

	resourcesMap := map[string]map[string]interface{}{}

	for _, tableInfo := range config.Tables {

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		resource := make(map[string]interface{})

		resource["displayName"] = tableInfo.TableName
		resource["description"] = "Resources in this group are related to " + tableInfo.TableName

		// BEGIN: POST request

		postMethod := make(map[string]interface{})
		postMethod["displayName"] = fmt.Sprintf("Create new %s", tableInfo.TableName)
		postBody := make(map[string]interface{})
		postBody["type"] = tableInfo.TableName
		postMethod["body"] = postBody
		postResponseMap := make(map[string]interface{})
		postOkResponse := make(map[string]interface{})
		postResponseBody := make(map[string]interface{})
		postResponseBody["type"] = "object"
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
		postResponseBody = map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": dataInResponse,
				"links": map[string]interface{}{
					"type": "PaginationStatus",
				},
			},
		}
		postOkResponse["body"] = postResponseBody
		postResponseMap["200"] = postOkResponse
		postMethod["responses"] = postResponseMap
		resource["post"] = &postMethod
		//  END: POST Request

		//  BEGIN: GET Request

		getAllMethod := make(map[string]interface{})
		getAllMethod["description"] = fmt.Sprintf("Returns a list of %v", tableInfo.TableName)
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
			"type": "object",
			"properties": map[string]interface{}{
				"data": dataInResponse,
				"links": map[string]interface{}{
					"type": "PaginationStatus",
				},
			},
		}
		getResponseMap["200"] = get200Response
		getAllMethod["responses"] = getResponseMap
		resource["get"] = &getAllMethod
		//  END: GET Request

		//fakeObject := fakerservice.NewFakeInstance(tableInfo)

		//  BEGIN: GET ById Request

		byIdResource := make(map[string]interface{})
		nestedMap := make(map[string]interface{})
		byIdResource["uriParameters"] = map[string]interface{}{
			"referenceId": map[string]interface{}{
				"type":        "string",
				"description": "Reference id of the " + tableInfo.TableName + " to be fetched",
				"required":    true,
			},
		}
		getByIdMethod := make(map[string]interface{})
		getByIdMethod200Response := make(map[string]interface{})
		getByIdMethod200Response["body"] = map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"data": dataInResponse,
				"links": map[string]interface{}{
					"type": "PaginationStatus",
				},
			},
		}

		getByIdResponseMap := make(map[string]interface{})
		getByIdResponseMap["200"] = getByIdMethod200Response
		getByIdMethod["responses"] = getByIdResponseMap
		byIdResource["get"] = getByIdMethod
		nestedMap["/{referenceId}"] = byIdResource

		//  END: GET ById Request

		// BEGIN: POST request

		patchMethod := make(map[string]interface{})
		patchMethod["displayName"] = fmt.Sprintf("Edit an existing %s", tableInfo.TableName)
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
		patchOkResponse["body"] = patchResponseBody
		postResponseMap["200"] = patchOkResponse
		patchMethod["responses"] = patchResponseMap
		resource["patch"] = &patchMethod
		//  END: PATCH Request

		// BEGIN: DELETE Request

		deleteByIdResource := make(map[string]interface{})
		deleteByIdResource["uriParameters"] = map[string]interface{}{
			"referenceId": map[string]interface{}{
				"type":        "string",
				"description": "Reference id of the " + tableInfo.TableName + " to be fetched",
				"required":    true,
			},
		}
		deleteByIdMethod := make(map[string]interface{})
		deleteByIdMethod200Response := make(map[string]interface{})
		deleteByIdMethod200Response["body"] = map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": "string",
			},
		}

		deleteByIdResponseMap := make(map[string]interface{})
		deleteByIdResponseMap["200"] = deleteByIdMethod200Response
		deleteByIdMethod["responses"] = deleteByIdResponseMap
		deleteByIdResource["get"] = deleteByIdMethod
		nestedMap["/{referenceId}"] = deleteByIdResource

		// END: DELETE Request

		for k, v := range nestedMap {
			resource[k] = v
		}

		if tableInfo.IsStateTrackingEnabled {

			//tableInfo.StateMachines

		}

		resourcesMap["/api/"+tableInfo.TableName] = resource

	}


	for key, val := range resourcesMap {
		apiDefinition[key] = val
	}

	ym, _ := yaml.Marshal(apiDefinition)
	return "#%RAML 1.0\n" + string(ym)

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
