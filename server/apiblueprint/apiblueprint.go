package apiblueprint

import (
	"bytes"
	"github.com/artpar/goms/server/resource"
	"github.com/artpar/api2go"

	"fmt"
	"strings"
	//"github.com/artpar/goms/server/fakerservice"
	"github.com/artpar/go-raml/raml"
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
	"deleted_at": true,
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

	apiDefinition := raml.APIDefinition{}

	blueprintWriter := NewBluePrintWriter()

	blueprintWriter.WriteString("#%RAML 1.0")
	blueprintWriter.WriteString("")

	apiDefinition.Title = "Goms server"
	apiDefinition.Version = "v1"
	apiDefinition.BaseURI = fmt.Sprintf("http://%v", config.Hostname)
	apiDefinition.MediaType = "application/json"
	apiDefinition.Protocols = []string{"HTTP", "HTTPS"}

	typeMap := make(map[string]raml.Type)

	var relatedStructureType raml.Type
	relatedStructureType.Type = "object"
	relatedStructureType.Properties = map[string]interface{}{
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

	var paginationObject raml.Type
	paginationObject.Type = "object"
	paginationObject.Properties = map[string]interface{}{
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

	var paginationStatus raml.Type
	paginationStatus.Type = "object"
	paginationStatus.Properties = map[string]interface{}{
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

	var IncludedRelationship raml.Type
	IncludedRelationship.Type = "object"
	IncludedRelationship.Properties = map[string]interface{}{
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
		var ramlType raml.Type

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		properties := make(map[string]interface{})
		ramlType.Type = "object"
		blueprintWriter.WriteString("  " + tableInfo.TableName + ":")
		blueprintWriter.WriteString("    type: object")
		blueprintWriter.WriteString("    properties:")

		for _, colInfo := range tableInfo.Columns {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			properties[colInfo.ColumnName] = CreateColumnLine(colInfo)
		}

		for _, relation := range tableInfo.Relations {
			if relation.Subject == tableInfo.TableName {
				properties[relation.GetObjectName()] = relation.GetObject()
			} else {
				properties[relation.GetSubjectName()] = relation.GetSubject()
			}
		}

		ramlType.Properties = properties
		typeMap[tableInfo.TableName] = ramlType
	}

	apiDefinition.Types = typeMap

	resourcesMap := map[string]raml.Resource{}

	for _, tableInfo := range config.Tables {

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		var resource raml.Resource

		resource.DisplayName = tableInfo.TableName
		resource.Description = "Resources in this group are related to " + tableInfo.TableName
		var postMethod raml.Method

		postMethod.DisplayName = fmt.Sprintf("Create new %s", tableInfo.TableName)

		var postBody raml.Bodies

		postBody.Type = tableInfo.TableName
		postMethod.Bodies = postBody

		postResponseMap := make(map[raml.HTTPCode]raml.Response)

		var postOkResponse raml.Response

		var postResponseBody raml.Bodies
		postResponseBody.Type = "object"
		forMimeTypeMap := make(map[string]raml.Body)

		relationshipMap := make(map[string]interface{}, 0)

		for _, relation := range tableInfo.Relations {
			if relation.Object == tableInfo.TableName {
				relationshipMap[relation.SubjectName] = "ReferenceToIncludedObject"
			} else {
				relationshipMap[relation.ObjectName] = "ReferenceToIncludedObject"
			}
		}

		var dataInResponse = &raml.BodiesProperty{
			Type: "object",
			Properties: map[string]interface{}{
				"attributes": raml.BodiesProperty{
					Type: tableInfo.TableName,
				},
				"id": raml.BodiesProperty{
					Type: "string",
				},
				"type": raml.BodiesProperty{
					Type: "string",
				},
				"relationships": raml.BodiesProperty{
					Type:       "object",
					Properties: relationshipMap,
				},
			},
		}

		postResponseBody.ApplicationJSON = &raml.BodiesProperty{
			Type: "object",
			Properties: map[string]interface{}{
				"data": dataInResponse,
				"links": raml.BodiesProperty{
					Type: "PaginationStatus",
				},
			},
		}

		var postCreationContent raml.Body

		forMimeTypeMap["application/json"] = postCreationContent

		postOkResponse.Bodies = postResponseBody

		postResponseMap["200"] = postOkResponse
		postMethod.Responses = postResponseMap

		resource.Post = &postMethod
		//  END POST Request

		//  BEGIN GET Request

		var getAllMethod raml.Method
		getAllMethod.Description = fmt.Sprintf("Returns a list of %v", tableInfo.TableName)
		getAllMethod.DisplayName = fmt.Sprintf("Get " + tableInfo.TableName)

		getAllMethod.QueryParameters = map[string]raml.NamedParameter{
			"sort": {
				Type:        "string",
				Required:    false,
				Description: "Field name to sort by",
			},
			"page%5Bnumber%5D": {
				Type:        "string",
				Required:    false,
				Description: "Page number for the query set, starts with 1",
			},
			"page%5Bsize%5D": {
				Type:        "string",
				Required:    false,
				Description: "Size of one page, try 10",
			},
			"query": {
				Type:        "string",
				Required:    false,
				Description: "search text in indexed columns",
			},
		}

		getResponseMap := make(map[raml.HTTPCode]raml.Response)

		var get200Response raml.Response

		get200Response.Bodies = raml.Bodies{
			ApplicationJSON: &raml.BodiesProperty{
				Type: "object",
				Properties: map[string]interface{}{
					"data": dataInResponse,
					"links": raml.BodiesProperty{
						Type: "PaginationStatus",
					},
				},
			},
		}

		getResponseMap["200"] = get200Response
		getAllMethod.Responses = getResponseMap

		resource.Get = &getAllMethod

		//fakeObject := fakerservice.NewFakeInstance(tableInfo)

		var byIdResource raml.Resource

		nestedMap := make(map[string]*raml.Resource)

		byIdResource.URIParameters = map[string]raml.NamedParameter{
			"referenceId": raml.NamedParameter{
				Type:        "string",
				Description: "Reference id of the " + tableInfo.TableName + "to be fetched",
				Required:    true,
			},
		}

		nestedMap["/{referenceId}"] = &byIdResource

		var getByIdMethod raml.Method

		var getByIdMethod200Response raml.Response
		getByIdMethod200Response.Bodies = raml.Bodies{
			ApplicationJSON: &raml.BodiesProperty{
				Type: "object",
				Properties: map[string]interface{}{
					"data": dataInResponse,
					"links": raml.BodiesProperty{
						Type: "PaginationStatus",
					},
				},
			},
		}

		getByIdResponseMap := make(map[raml.HTTPCode]raml.Response)
		getByIdResponseMap["200"] = getByIdMethod200Response
		getByIdMethod.Responses = getByIdResponseMap

		byIdResource.Get = &getByIdMethod

		resource.Nested = nestedMap
		resourcesMap["/api/"+tableInfo.TableName] = resource

		//blueprintWriter.WriteString("  /{referenceId}:")
		//blueprintWriter.WriteString("    uriParameters")
		//blueprintWriter.WriteString("      referenceId: ")
		//blueprintWriter.WriteString("        type: string")
		//blueprintWriter.WriteString("        description: reference id of the " + tableInfo.TableName + " to be fetched")
		//blueprintWriter.WriteString("        required: true")
		//blueprintWriter.WriteString("    get:")
		//blueprintWriter.WriteString("      description: Get a single " + tableInfo.TableName + " by reference id")
		//blueprintWriter.WriteString("      displayName: Returns the " + tableInfo.TableName)
		//
		//blueprintWriter.WriteString("      responses:")
		//blueprintWriter.WriteString("        200:")
		//blueprintWriter.WriteString("          body:")
		//blueprintWriter.WriteString(fmt.Sprintf("          data:"))
		//blueprintWriter.WriteString(fmt.Sprintf("            type: object"))
		//blueprintWriter.WriteString(fmt.Sprintf("            description: list of queried %s", tableInfo.TableName))
		//blueprintWriter.WriteString(fmt.Sprintf("            properties:"))
		//blueprintWriter.WriteString(fmt.Sprintf("              attributes:"))
		//blueprintWriter.WriteString(fmt.Sprintf("                type: object"))
		//blueprintWriter.WriteString(fmt.Sprintf("                description: Attributes of %s", tableInfo.TableName))
		//blueprintWriter.WriteString(fmt.Sprintf("              id: "))
		//blueprintWriter.WriteString(fmt.Sprintf("                type: string"))
		//blueprintWriter.WriteString(fmt.Sprintf("                description: - reference id of this %s", tableInfo.TableName))
		//blueprintWriter.WriteString(fmt.Sprintf("              relationships:"))
		//blueprintWriter.WriteString(fmt.Sprintf("                description:  - related entities of %v", tableInfo.TableName))
		//blueprintWriter.WriteString(fmt.Sprintf("                type: object"))
		//blueprintWriter.WriteString(fmt.Sprintf("                properties:"))

		//for _, relation := range tableInfo.Relations {
		//	if tableInfo.TableName == relation.Object {
		//
		//		blueprintWriter.WriteString(fmt.Sprintf("                  %s: ", relation.SubjectName))
		//
		//		if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
		//			blueprintWriter.WriteString(fmt.Sprintf("                    type: ReferenceToIncludedObject"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                    description: Reference to related %s of %s", relation.SubjectName, tableInfo.TableName))
		//		} else {
		//			blueprintWriter.WriteString(fmt.Sprintf("                    type: ReferenceToIncludedObject[]"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                    description: References to related %s of %s", relation.SubjectName, tableInfo.TableName))
		//		}
		//	} else {
		//		blueprintWriter.WriteString(fmt.Sprintf("                  %s: ", relation.ObjectName))
		//
		//		if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
		//			blueprintWriter.WriteString(fmt.Sprintf("                data:"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                  type: ReferenceToIncludedObject"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                  description: Reference to related %s of %s", relation.Object, tableInfo.TableName))
		//		} else {
		//			blueprintWriter.WriteString(fmt.Sprintf("                data:"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                  type: ReferenceToIncludedObject[]"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                  description: References to related %s of %s", relation.Object, tableInfo.TableName))
		//		}
		//		blueprintWriter.WriteString(fmt.Sprintf("                links:"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                  type: object"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                  description: Urls to fetch associated objects"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                  properties:"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                    related:"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                      type: string"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                      descriptionUrls to Fetch relations of %s", relation.Object))
		//		blueprintWriter.WriteString(fmt.Sprintf("                    self:"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                      type: string"))
		//		blueprintWriter.WriteString(fmt.Sprintf("                      description: Url to Fetch self %s", relation.Object))
		//	}
		//}

		//blueprintWriter.WriteString(fmt.Sprintf("              type:"))
		//blueprintWriter.WriteString(fmt.Sprintf("                type: string"))
		//blueprintWriter.WriteString(fmt.Sprintf("                description: this is the type name returned along with each object, will be %s here", tableInfo.TableName))
		//blueprintWriter.WriteString(fmt.Sprintf("            included:"))
		//blueprintWriter.WriteString(fmt.Sprintf("              type: array"))
		//blueprintWriter.WriteString(fmt.Sprintf("              items: "))
		//blueprintWriter.WriteString(fmt.Sprintf("                type: object"))
		//blueprintWriter.WriteString(fmt.Sprintf("              relationships:"))
		//blueprintWriter.WriteString(fmt.Sprintf("                type:"))
		//blueprintWriter.WriteString(fmt.Sprintf("                  type: array"))
		//blueprintWriter.WriteString(fmt.Sprintf("                  items:"))
		//blueprintWriter.WriteString(fmt.Sprintf("                    type: object"))
		//blueprintWriter.WriteString(fmt.Sprintf("                  description: Type of the related object"))
		//blueprintWriter.WriteString(fmt.Sprintf("            links:"))
		//blueprintWriter.WriteString(fmt.Sprintf("              type: PaginationStatus"))
		//blueprintWriter.WriteString("")
		//
		//blueprintWriter.WriteString("    patch:")
		//blueprintWriter.WriteString(fmt.Sprintf("    description: Edit existing %s", tableInfo.TableName))
		//blueprintWriter.WriteString("      body: ")
		//blueprintWriter.WriteString("      type: " + tableInfo.TableName)
		//blueprintWriter.WriteString("        responses: ")
		//blueprintWriter.WriteString("          200: ")
		//blueprintWriter.WriteString("            body: ")
		//blueprintWriter.WriteString("              type: object")
		//blueprintWriter.WriteString("              properties:")
		//blueprintWriter.WriteString("                data:")
		//blueprintWriter.WriteString("                  type: object")
		//blueprintWriter.WriteString("                    properties:")
		//blueprintWriter.WriteString("                      attributes: " + tableInfo.TableName)
		//blueprintWriter.WriteString("                      id: ")
		//blueprintWriter.WriteString("                        type: string")
		//blueprintWriter.WriteString("                      type: ")
		//blueprintWriter.WriteString("                        type: string")
		//blueprintWriter.WriteString("                      relations:")
		//
		//for _, relation := range tableInfo.Relations {
		//	if relation.Object == tableInfo.TableName {
		//		blueprintWriter.WriteString(fmt.Sprintf("                        %v: ReferenceToIncludedObject", relation.SubjectName))
		//	} else {
		//		blueprintWriter.WriteString(fmt.Sprintf("                        %v: ReferenceToIncludedObject", relation.ObjectName))
		//	}
		//}
		//
		//blueprintWriter.WriteString("    delete:")
		//blueprintWriter.WriteString(fmt.Sprintf("      description: Delete an existing %s", tableInfo.TableName))

		//for _, relation := range tableInfo.Relations {
		//
		//	var name, typeName string
		//	if relation.Object == tableInfo.TableName {
		//		name = relation.SubjectName
		//		typeName = relation.Subject
		//	} else {
		//		name = relation.ObjectName
		//		typeName = relation.Object
		//	}
		//
		//	blueprintWriter.WriteString("    /" + name)
		//	blueprintWriter.WriteString("      description: " + typeName + " associated with " + tableInfo.TableName)
		//	blueprintWriter.WriteString("+ Parameters")
		//	blueprintWriter.WriteString("    + sort (optional, string) - sort results by a columns")
		//	blueprintWriter.WriteString("    + page%5Bnumber%5D (string, required) - Page number for the query set, starts with 1")
		//	blueprintWriter.WriteString("    + page%5Bsize%5D (string, required) - Size of one page, try 10")
		//	blueprintWriter.WriteString("    + query (optional, string) - sort results by a columns")
		//	blueprintWriter.WriteString("    + referenceId (string, required) - reference id of the parent object as path param")
		//	blueprintWriter.WriteString("")
		//
		//	blueprintWriter.WriteString("## Fetch related " + typeName + " which are of type " + name + " [GET]")
		//	blueprintWriter.WriteString(fmt.Sprintf("Fetch related %s", typeName))
		//	blueprintWriter.WriteString("")
		//
		//	blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
		//	blueprintWriter.WriteString("")
		//	blueprintWriter.WriteString("    + Body")
		//	blueprintWriter.WriteString("")
		//	blueprintWriter.WriteString(fmt.Sprintf("    + Attributes (object)"))
		//	blueprintWriter.WriteString(fmt.Sprintf("            + data - list of queried %s", typeName))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + attributes (%s) - Attributes of %s", typeName, name))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + id (string) - reference id of this %s", typeName))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + relationships - related entities of %v", typeName))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + type (string) - type of this object"))
		//	blueprintWriter.WriteString(fmt.Sprintf("            + included - Array of included related entities to %v", typeName))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + attributes (object) - Attributes of the related entity"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + relationships Links to all the relations"))
		//
		//	subRelations := tableMap[typeName].Relations
		//	for _, subRelation := range subRelations {
		//		if tableInfo.TableName == subRelation.Object {
		//
		//			blueprintWriter.WriteString(fmt.Sprintf("                    + %s (object)", subRelation.SubjectName))
		//
		//			if subRelation.Relation == "belongs_to" || subRelation.Relation == "has_one" {
		//				blueprintWriter.WriteString(fmt.Sprintf("                        + data (RelationStructure) - Reference to related %s of %s", subRelation.SubjectName, subRelation.Subject, typeName))
		//			} else {
		//				blueprintWriter.WriteString(fmt.Sprintf("                        + data (array[RelationStructure]) - References to related %s of %s", subRelation.SubjectName, subRelation.Subject, typeName))
		//			}
		//		} else {
		//			blueprintWriter.WriteString(fmt.Sprintf("                    + %s (object)", subRelation.ObjectName))
		//
		//			if subRelation.Relation == "belongs_to" || subRelation.Relation == "has_one" {
		//				blueprintWriter.WriteString(fmt.Sprintf("                        + data (RelationStructure) - Reference to related %s of %s", subRelation.Object, typeName))
		//			} else {
		//				blueprintWriter.WriteString(fmt.Sprintf("                        + data (array[RelationStructure]) - References to related %s of %s", subRelation.Object, typeName))
		//			}
		//			blueprintWriter.WriteString(fmt.Sprintf("                        + links (object) - Urls to fetch associated objects"))
		//			blueprintWriter.WriteString(fmt.Sprintf("                            + related (string) - Urls to Fetch relations of %s", subRelation.Object))
		//			blueprintWriter.WriteString(fmt.Sprintf("                            + self (string) - Url to Fetch self %s", subRelation.Object))
		//		}
		//	}
		//
		//	blueprintWriter.WriteString(fmt.Sprintf("                + type (string) - type of this included object"))
		//	blueprintWriter.WriteString(fmt.Sprintf("            + links (object)"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + current_page (number) - The current page, for pagination"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + from (number) - Index of the first records fetched in this result"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + last_page (number) - The last page number in current query set"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + per_page (number) - This is the number of results in one page"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + to (number) - Index of the last record feched in this result"))
		//	blueprintWriter.WriteString(fmt.Sprintf("                + total (number) - Total number of records"))
		//	blueprintWriter.WriteString("")
		//
		//}

	}
	apiDefinition.Resources = resourcesMap

	ym, _ := yaml.Marshal(apiDefinition)
	return string(ym)

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
