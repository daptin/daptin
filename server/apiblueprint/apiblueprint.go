package apiblueprint

import (
	"bytes"
	"encoding/json"
	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
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
	
	// Add description if available
	if colInfo.ColumnDescription != "" {
		m["description"] = colInfo.ColumnDescription
	}
	
	// Add default value if specified
	if colInfo.DefaultValue != "" && colInfo.DefaultValue != "null" {
		m["default"] = colInfo.DefaultValue
	}
	
	// Add format based on column type
	switch columnType {
	case "email":
		m["format"] = "email"
		m["example"] = "user@example.com"
	case "date":
		m["format"] = "date"
		m["example"] = "2024-01-15"
	case "datetime":
		m["format"] = "date-time"
		m["example"] = "2024-01-15T09:30:00Z"
	case "password":
		m["format"] = "password"
		m["writeOnly"] = true
	case "url":
		m["format"] = "uri"
		m["example"] = "https://example.com"
	case "uuid":
		m["format"] = "uuid"
		m["example"] = "550e8400-e29b-41d4-a716-446655440000"
	}
	
	// Add enum values if available
	if len(colInfo.Options) > 0 {
		enumValues := make([]string, 0)
		for _, option := range colInfo.Options {
			if strValue, ok := option.Value.(string); ok {
				enumValues = append(enumValues, strValue)
			} else if option.Value != nil {
				enumValues = append(enumValues, fmt.Sprintf("%v", option.Value))
			}
		}
		if len(enumValues) > 0 {
			m["enum"] = enumValues
		}
	}
	
	// Add nullable property for clarity
	if colInfo.IsNullable {
		m["nullable"] = true
	}
	
	return m
}

func BuildApiBlueprint(config *resource.CmsConfig, cruds map[string]*resource.DbResource) string {

	tableMap := map[string]table_info.TableInfo{}
	for _, table := range config.Tables {
		tableMap[table.TableName] = table
	}

	// Use yaml.MapSlice to preserve key order
	apiDefinition := yaml.MapSlice{
		{Key: "openapi", Value: "3.0.0"},
	}
	
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "info",
		Value: map[string]interface{}{
		"version": "1.0.0",
		"title":   "Daptin API endpoint",
		"license": map[string]interface{}{
			"name": "MIT",
			"url": "https://opensource.org/licenses/MIT",
		},
		"contact": map[string]interface{}{
			"name":  "Daptin Support",
			"url":   "https://dapt.in",
			"email": "artpar@gmail.com",
		},
		"description": `Daptin REST API server provides a complete backend with CRUD operations, authentication, authorization, and custom actions. This API follows JSON:API specification for resource representation.

## Key Features
- **CRUD Operations**: Full Create, Read, Update, Delete operations on all entities
- **Authentication**: JWT and Basic authentication support
- **Authorization**: Role-based access control with Guest, User, and Group permissions
- **Relationships**: Support for has_one, has_many, and many_to_many relationships
- **Actions**: Custom actions on resources with input validation
- **Pagination**: Offset and cursor-based pagination
- **Filtering**: Advanced filtering with JSON-based queries
- **Rate Limiting**: Built-in rate limiting per endpoint
- **State Machines**: Workflow support with state transitions

## Rate Limiting
All endpoints are rate-limited. Rate limit information is returned in response headers:
- X-RateLimit-Limit: Maximum requests allowed
- X-RateLimit-Remaining: Requests remaining
- X-RateLimit-Reset: Unix timestamp when limit resets`,
		"x-logo": map[string]interface{}{
			"url": "https://daptin.github.io/daptin/images/logo.png",
			"altText": "Daptin Logo",
		},
	},
	})

	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "servers",
		Value: []map[string]interface{}{
		{
			"url":         fmt.Sprintf("http://%v", config.Hostname),
			"description": "Server " + config.Hostname,
		},
	},
	})
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
		"description": "Response from an action execution. Actions can produce multiple outcomes, each with a specific response type and attributes.",
		"required": []string{"ResponseType", "Attributes"},
		"properties": map[string]interface{}{
			"ResponseType": map[string]interface{}{
				"type": "string",
				"description": "The type of response produced by the action outcome",
				"enum": []string{
					"client.redirect",
					"client.notify", 
					"client.file.download",
					"client.token.set",
					"client.store.set",
					"client.script.run",
				},
				"example": "client.notify",
			},
			"Attributes": map[string]interface{}{
				"type": "object",
				"description": "Response-specific attributes. The structure varies based on ResponseType.",
				"additionalProperties": true,
				"examples": []map[string]interface{}{
					{
						"type": "success",
						"title": "Operation Successful",
						"message": "The action completed successfully",
					},
					{
						"location": "/dashboard",
						"delay": 2000,
					},
					{
						"name": "export.csv",
						"content": "base64_encoded_content",
						"contentType": "text/csv",
					},
				},
			},
		},
		"example": map[string]interface{}{
			"ResponseType": "client.notify",
			"Attributes": map[string]interface{}{
				"type": "success",
				"title": "Success",
				"message": "Action executed successfully",
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
			"description": "Index of the last record fetched in this result",
		},
		"total": map[string]interface{}{
			"type":        "number",
			"description": "Total number of records",
		},
	}
	typeMap["PaginationStatus"] = paginationStatus
	typeMap["ActionResponse"] = actionResponse
	
	// Add comprehensive error response schemas
	errorResponse := map[string]interface{}{
		"type": "object",
		"required": []string{"errors"},
		"properties": map[string]interface{}{
			"errors": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"required": []string{"status", "title"},
					"properties": map[string]interface{}{
						"id": map[string]interface{}{
							"type": "string",
							"description": "A unique identifier for this particular occurrence of the problem",
						},
						"status": map[string]interface{}{
							"type": "string",
							"description": "The HTTP status code applicable to this problem",
							"example": "400",
						},
						"code": map[string]interface{}{
							"type": "string",
							"description": "An application-specific error code",
							"example": "VALIDATION_ERROR",
						},
						"title": map[string]interface{}{
							"type": "string",
							"description": "A short, human-readable summary of the problem",
							"example": "Validation failed",
						},
						"detail": map[string]interface{}{
							"type": "string",
							"description": "A human-readable explanation specific to this occurrence",
							"example": "The email field must be a valid email address",
						},
						"source": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"pointer": map[string]interface{}{
									"type": "string",
									"description": "JSON Pointer to the associated entity in the request",
									"example": "/data/attributes/email",
								},
								"parameter": map[string]interface{}{
									"type": "string",
									"description": "String indicating which query parameter caused the error",
									"example": "filter",
								},
							},
						},
					},
				},
			},
		},
	}
	typeMap["ErrorResponse"] = errorResponse
	
	// Add rate limit error response
	rateLimitResponse := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"errors": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{
							"type": "string",
							"example": "429",
						},
						"title": map[string]interface{}{
							"type": "string",
							"example": "Too Many Requests",
						},
						"detail": map[string]interface{}{
							"type": "string",
							"example": "Rate limit exceeded. Please retry after some time.",
						},
					},
				},
			},
			"meta": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"rate_limit": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"limit": map[string]interface{}{
								"type": "integer",
								"description": "The maximum number of requests allowed",
							},
							"remaining": map[string]interface{}{
								"type": "integer",
								"description": "The number of requests remaining in the current window",
							},
							"reset": map[string]interface{}{
								"type": "integer",
								"description": "Unix timestamp when the rate limit window resets",
							},
						},
					},
				},
			},
		},
	}
	typeMap["RateLimitResponse"] = rateLimitResponse

	IncludedRelationship := make(map[string]interface{})
	IncludedRelationship["type"] = "object"
	IncludedRelationship["description"] = "Relationship object following JSON:API specification"
	IncludedRelationship["properties"] = map[string]interface{}{
		"data": map[string]interface{}{
			"oneOf": []map[string]interface{}{
				{
					"$ref": "#/components/schemas/RelatedStructure",
					"description": "Single related resource (has_one/belongs_to)",
				},
				{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/components/schemas/RelatedStructure",
					},
					"description": "Multiple related resources (has_many)",
				},
			},
		},
		"links": map[string]interface{}{
			"type":        "object",
			"description": "Links to fetch or manipulate the relationship",
			"properties": map[string]interface{}{
				"related": map[string]interface{}{
					"type":        "string",
					"format":      "uri",
					"description": "URL to fetch the related resource(s)",
					"example":     "/api/posts/123/author",
				},
				"self": map[string]interface{}{
					"type":        "string",
					"format":      "uri",
					"description": "URL to fetch the relationship itself",
					"example":     "/api/posts/123/relationships/author",
				},
			},
		},
		"meta": map[string]interface{}{
			"type":        "object",
			"description": "Additional metadata about the relationship",
			"additionalProperties": true,
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
		
		// Add table description if available
		if tableInfo.TableDescription != "" {
			ramlType["description"] = tableInfo.TableDescription
		}
		
		// Add example object
		exampleObj := make(map[string]interface{})
		for colName, colDef := range properties {
			if colMap, ok := colDef.(map[string]interface{}); ok {
				if example, exists := colMap["example"]; exists {
					exampleObj[colName] = example
				} else if colMap["type"] == "string" {
					exampleObj[colName] = "example " + colName
				} else if colMap["type"] == "number" {
					exampleObj[colName] = 42
				} else if colMap["type"] == "boolean" {
					exampleObj[colName] = true
				}
			}
		}
		if len(exampleObj) > 0 {
			ramlType["example"] = exampleObj
		}

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
		ramlActionType["type"] = "object"

		// Generate comprehensive description for the action schema
		schemaDescription := generateActionSchemaDescription(action)
		ramlActionType["description"] = schemaDescription

		actionProperties := make(map[string]interface{})
		requiredFields := []string{}
		
		for _, colInfo := range action.InFields {
			if colInfo.IsForeignKey {
				continue
			}
			if skipColumns[colInfo.ColumnName] {
				continue
			}

			// Create enhanced column definition with better descriptions
			colDef := CreateColumnLine(colInfo)
			
			// Add field-specific descriptions based on the action context
			if desc, ok := getFieldDescription(action.Name, colInfo.ColumnName); ok {
				colDef["description"] = desc
			}
			
			actionProperties[colInfo.ColumnName] = colDef
			
			// Track required fields
			if !colInfo.IsNullable {
				requiredFields = append(requiredFields, colInfo.ColumnName)
			}
		}
		
		if !action.InstanceOptional {
			actionProperties[action.OnType+"_id"] = map[string]interface{}{
				"type":        "string",
				"format":      "uuid",
				"description": fmt.Sprintf("Reference ID of the %s instance on which to execute this action. This must be a valid UUID of an existing %s record.", action.OnType, action.OnType),
				"example":     "550e8400-e29b-41d4-a716-446655440000",
			}
			requiredFields = append(requiredFields, action.OnType+"_id")
		}

		ramlActionType["properties"] = actionProperties
		
		if len(requiredFields) > 0 {
			ramlActionType["required"] = requiredFields
		}
		
		// Add example object for the action
		if example := generateActionExample(action); example != nil {
			ramlActionType["example"] = example
		}
		
		typeMap[fmt.Sprintf("%sOn%sRequestObject", strcase.ToCamel(action.Name), strcase.ToCamel(action.OnType))] = ramlActionType

	}

	resourcesMap := map[string]map[string]interface{}{}
	tableInfoMap := make(map[string]table_info.TableInfo)
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
				relatedTable, exists := tableInfoMap[rel.Object]
				if !exists || relatedTable.TableName == "" {
					log.Printf("Warning: Related table '%s' not found for relation %v", rel.Object, rel)
					continue
				}
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				getMethod["description"] = "Returns a list of all " + ProperCase(relatedTable.TableName) + " related to a " + tableInfo.TableName
				getMethod["operationId"] = "Get" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(rel.SubjectName)
				getMethod["summary"] = "Fetch related " + rel.ObjectName + " of " + tableInfo.TableName

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
				deleteMethod["description"] = fmt.Sprintf("Remove a related %v from the %v", tableInfo.TableName, rel.ObjectName)
				deleteMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "delete"}

				deleteMethod["summary"] = fmt.Sprintf("Delete related %s of %v", rel.ObjectName, tableInfo.TableName)
				deleteMethod["operationId"] = "Delete" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)

				relationsById["get"] = getMethod
				relationsById["delete"] = deleteMethod

				patchMethod := CreatePatchRelationMethod(relatedTable)
				patchMethod["description"] = fmt.Sprintf("Add a related %v from the %v", tableInfo.TableName, rel.ObjectName)
				patchMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "patch"}

				patchMethod["summary"] = fmt.Sprintf("Add related %s of %v", rel.ObjectName, tableInfo.TableName)
				patchMethod["operationId"] = "Patch" + strcase.ToCamel(rel.ObjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)

				relationsById["patch"] = patchMethod

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
				relatedTable, exists := tableInfoMap[rel.Subject]
				if !exists || relatedTable.TableName == "" {
					log.Printf("Warning: Related table '%s' not found for relation %v", rel.Subject, rel)
					continue
				}
				getMethod := CreateGetAllMethod(relatedTable, CreateDataInResponse(relatedTable))
				getMethod["summary"] = "Related " + strcase.ToCamel(rel.SubjectName) + " of a " + strcase.ToCamel(tableInfo.TableName)
				getMethod["operationId"] = "Related" + strcase.ToCamel(rel.SubjectName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				patchMethod["summary"] = fmt.Sprintf("Fetch related %s of %v", rel.SubjectName, tableInfo.TableName)
				patchMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "get"}

				deleteMethod := CreateDeleteRelationMethod(relatedTable)
				deleteMethod["description"] = fmt.Sprintf("Remove a related %v from the %v", rel.SubjectName, rel.ObjectName)
				deleteMethod["operationId"] = "Delete" + strcase.ToCamel(relatedTable.TableName) + "Of" + strcase.ToCamel(tableInfo.TableName)
				patchMethod["summary"] = fmt.Sprintf("Delete related %s of %v", rel.SubjectName, tableInfo.TableName)
				patchMethod["tags"] = []string{rel.ObjectName, rel.Subject, rel.SubjectName, rel.Object, rel.Relation, "delete"}
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
		// Determine appropriate tags based on action name and type
		actionTags := []string{action.OnType}
		actionCategory := categorizeAction(action.Name)
		if actionCategory != "" {
			actionTags = append(actionTags, actionCategory)
		}

		// Create detailed description for the action
		actionDescription := generateActionDescription(action)

		// Generate example request body
		exampleRequest := generateActionRequestExample(action)

		resourcesMap[fmt.Sprintf("/action/%s/%s", action.OnType, action.Name)] = map[string]interface{}{
			"post": map[string]interface{}{
				"tags":        actionTags,
				"operationId": "Execute" + strcase.ToCamel(action.Name) + "ActionOn" + strcase.ToCamel(action.OnType),
				"summary":     action.Label,
				"description": actionDescription,
				"x-codeSamples": []map[string]interface{}{
					{
						"lang": "curl",
						"source": generateCurlExample(action),
					},
				},
				"requestBody": map[string]interface{}{
					"required":    len(action.InFields) > 0,
					"description": fmt.Sprintf("Request body for %s action", action.Label),
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": map[string]interface{}{
								"$ref": "#/components/schemas/" + fmt.Sprintf("%sOn%sRequestObject", strcase.ToCamel(action.Name), strcase.ToCamel(action.OnType)),
							},
							"example": exampleRequest,
						},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": fmt.Sprintf("Successful execution of %s action", action.Label),
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "array",
									"description": "Array of action responses, each representing an outcome of the action",
									"items": map[string]interface{}{
										"$ref": "#/components/schemas/ActionResponse",
									},
								},
								"example": generateActionResponseExample(action),
							},
						},
					},
					"400": map[string]interface{}{
						"$ref": "#/components/responses/BadRequest",
					},
					"401": map[string]interface{}{
						"$ref": "#/components/responses/Unauthorized",
					},
					"403": map[string]interface{}{
						"$ref": "#/components/responses/Forbidden",
					},
					"422": map[string]interface{}{
						"$ref": "#/components/responses/UnprocessableEntity",
					},
					"429": map[string]interface{}{
						"$ref": "#/components/responses/TooManyRequests",
					},
				},
			},
		}

	}

	actionResource := make(map[string]interface{})

	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key:   "paths",
		Value: resourcesMap,
	})
	for n, v := range actionResource {
		apiDefinition = append(apiDefinition, yaml.MapItem{
			Key:   n,
			Value: v,
		})
	}

	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "components",
		Value: map[string]interface{}{
			"schemas": typeMap,
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description": `JWT Bearer token authentication. Obtain tokens via POST /auth/signin.

Permission model:
- **Guest**: Basic read permissions (GuestPeek, GuestRead)
- **User**: Full CRUD on owned resources (UserCRUD)
- **Group**: Shared permissions within groups (GroupCRUD)
- **Execute**: Permission to run actions

Example: Authorization: Bearer <your-jwt-token>`,
				},
				"basicAuth": map[string]interface{}{
					"type":        "http",
					"scheme":      "basic",
					"description": "Basic authentication using email and password",
				},
			},
			"parameters": CreateCommonParameters(),
			"responses": CreateCommonResponses(),
		},
	})
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "security",
		Value: []map[string][]string{
			{
				"bearerAuth": []string{},
			},
		},
	})
	
	// Add external documentation
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key: "externalDocs",
		Value: map[string]interface{}{
			"description": "Full Daptin Documentation",
			"url":         "https://docs.dapt.in",
		},
	})
	
	// Add tags for better organization
	tags := []map[string]interface{}{
		{
			"name":        "Authentication",
			"description": "Authentication endpoints for obtaining JWT tokens",
		},
		{
			"name":        "System Actions",
			"description": "System-level actions for managing Daptin instance configuration, data operations, and administrative tasks",
			"x-displayName": "System Actions",
		},
		{
			"name":        "Data Operations",
			"description": "Actions for importing, exporting, and manipulating data across tables",
			"x-displayName": "Data Operations",
		},
		{
			"name":        "Schema Management",
			"description": "Actions for managing database schema, tables, and columns",
			"x-displayName": "Schema Management",
		},
		{
			"name":        "Storage Management",
			"description": "Actions for managing cloud storage, file operations, and synchronization",
			"x-displayName": "Storage Management",
		},
		{
			"name":        "Certificate Management",
			"description": "Actions for generating and managing SSL/TLS certificates",
			"x-displayName": "Certificate Management",
		},
		{
			"name":        "User Management",
			"description": "Actions for user registration, authentication, and account management",
			"x-displayName": "User Management",
		},
	}
	
	// Add tags for each table
	for _, tableInfo := range config.Tables {
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}
		tag := map[string]interface{}{
			"name": tableInfo.TableName,
		}
		if tableInfo.TableDescription != "" {
			tag["description"] = tableInfo.TableDescription
		}
		tags = append(tags, tag)
	}
	
	apiDefinition = append(apiDefinition, yaml.MapItem{
		Key:   "tags",
		Value: tags,
	})

	ym, _ := yaml.Marshal(apiDefinition)
	return string(ym)

}

func CreateDataInResponse(tableInfo table_info.TableInfo) map[string]interface{} {
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
func CreatePostMethod(tableInfo table_info.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	postMethod := make(map[string]interface{})
	postMethod["operationId"] = fmt.Sprintf("Create%s", strcase.ToCamel(tableInfo.TableName))
	postMethod["summary"] = fmt.Sprintf("Create a new %v", tableInfo.TableName)
	postMethod["tags"] = []string{tableInfo.TableName, "create"}
	postBody := make(map[string]interface{})

	postBody["description"] = tableInfo.TableName + " to create"
	postBody["required"] = true
	postBody["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"required": []string{"data"},
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "object",
						"required": []string{"type", "attributes"},
						"properties": map[string]interface{}{
							"type": map[string]interface{}{
								"type":  "string",
								"enum": []string{tableInfo.TableName},
								"description": "Resource type identifier",
							},
							"attributes": map[string]interface{}{
								"$ref": "#/components/schemas/New" + strcase.ToCamel(tableInfo.TableName),
							},
							"relationships": map[string]interface{}{
								"type": "object",
								"description": "Related resources to create relationships with",
								"additionalProperties": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"data": map[string]interface{}{
											"type": "object",
											"properties": map[string]interface{}{
												"type": map[string]interface{}{
													"type": "string",
												},
												"id": map[string]interface{}{
													"type": "string",
													"format": "uuid",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			"example": CreatePostRequestExample(tableInfo),
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

	postResponseMap["201"] = postOkResponse
	postResponseMap["400"] = map[string]interface{}{
		"$ref": "#/components/responses/BadRequest",
	}
	postResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	postResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	postResponseMap["422"] = map[string]interface{}{
		"$ref": "#/components/responses/UnprocessableEntity",
	}
	postResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	postMethod["responses"] = postResponseMap
	return postMethod
}
func CreateGetAllMethod(tableInfo table_info.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
	getAllMethod := make(map[string]interface{})
	getAllMethod["description"] = "Returns a list of " + ProperCase(tableInfo.TableName)
	getAllMethod["operationId"] = "Get" + strcase.ToCamel(tableInfo.TableName)
	getAllMethod["summary"] = "List all " + tableInfo.TableName
	getAllMethod["tags"] = []string{tableInfo.TableName, "find", "get"}
	getAllMethod["parameters"] = []map[string]interface{}{
		{
			"$ref": "#/components/parameters/Sort",
		},
		{
			"$ref": "#/components/parameters/PageNumber",
		},
		{
			"$ref": "#/components/parameters/PageSize",
		},
		{
			"$ref": "#/components/parameters/Query",
		},
		{
			"$ref": "#/components/parameters/Filter",
		},
		{
			"$ref": "#/components/parameters/IncludedRelations",
		},
		{
			"$ref": "#/components/parameters/Fields",
		},
		{
			"name": "page[after]",
			"in":   "query",
			"schema": map[string]interface{}{
				"type":   "string",
				"format": "uuid",
			},
			"required":    false,
			"description": "Reference ID for cursor-based pagination. Returns results after this ID.",
			"example":     "550e8400-e29b-41d4-a716-446655440000",
		},
		{
			"name": "group",
			"in":   "query",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"required":    false,
			"description": "Base64 encoded JSON for grouping results. Example: {\"column\":\"category\",\"function\":\"count\"}",
			"example":     "eyJjb2x1bW4iOiJjYXRlZ29yeSIsImZ1bmN0aW9uIjoiY291bnQifQ==",
		},
		{
			"name": "accept",
			"in":   "header",
			"schema": map[string]interface{}{
				"type": "string",
				"enum": []string{"application/json", "text/csv", "application/xml"},
				"default": "application/json",
			},
			"required":    false,
			"description": "Response format. Supported: application/json (default), text/csv, application/xml",
		},
	}
	getResponseMap := make(map[string]interface{})
	get200Response := make(map[string]interface{})
	get200Response["description"] = "list of all " + tableInfo.TableName
	get200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"data": map[string]interface{}{
						"type": "array",
						"items": map[string]interface{}{
							"$ref": "#/components/schemas/" + strcase.ToCamel(tableInfo.TableName),
						},
					},
					"links": map[string]interface{}{
						"$ref": "#/components/schemas/PaginationStatus",
					},
					"included": map[string]interface{}{
						"type": "array",
						"description": "Included related resources when using included_relations parameter",
						"items": map[string]interface{}{
							"type": "object",
						},
					},
				},
			},
		},
		"text/csv": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "string",
				"description": "CSV formatted data. Use Accept: text/csv header.",
				"example": "id,name,email,created_at\n1,John Doe,john@example.com,2024-01-15T09:30:00Z\n",
			},
		},
		"application/xml": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "string",
				"description": "XML formatted data. Use Accept: application/xml header.",
				"example": "<data><item><id>1</id><name>John Doe</name></item></data>",
			},
		},
	}

	getResponseMap["200"] = get200Response
	getResponseMap["400"] = map[string]interface{}{
		"$ref": "#/components/responses/BadRequest",
	}
	getResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	getResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	getResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	getResponseMap["500"] = map[string]interface{}{
		"$ref": "#/components/responses/InternalServerError",
	}
	getAllMethod["responses"] = getResponseMap
	return getAllMethod
}
func ProperCase(str string) string {
	if len(str) == 0 {
		return ""
	}
	if len(str) == 1 {
		return strings.ToUpper(str)
	}
	st := str[1:]
	st = strings.Replace(st, "_", " ", -1)
	st = strings.Replace(st, ".", " ", -1)
	return strings.ToUpper(str[0:1]) + st
}

func CreateCommonParameters() map[string]interface{} {
	return map[string]interface{}{
		"PageNumber": map[string]interface{}{
			"name":        "page[number]",
			"in":          "query",
			"description": "Page number for pagination (1-based)",
			"schema": map[string]interface{}{
				"type":    "integer",
				"minimum": 1,
				"default": 1,
			},
			"example": 2,
		},
		"PageSize": map[string]interface{}{
			"name":        "page[size]",
			"in":          "query",
			"description": "Number of items per page",
			"schema": map[string]interface{}{
				"type":    "integer",
				"minimum": 1,
				"maximum": 100,
				"default": 20,
			},
			"example": 20,
		},
		"Sort": map[string]interface{}{
			"name":        "sort",
			"in":          "query",
			"description": "Sort fields. Use - prefix for descending order. Multiple fields can be comma-separated.",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "-created_at,name",
		},
		"Filter": map[string]interface{}{
			"name":        "filter",
			"in":          "query",
			"description": "JSON-based filtering. Supports operators: eq, ne, gt, gte, lt, lte, like, in, between",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": `{"name":{"like":"%john%"},"age":{"gte":18}}`,
		},
		"Query": map[string]interface{}{
			"name":        "query",
			"in":          "query",
			"description": "Full-text search across all indexed text columns",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "search term",
		},
		"IncludedRelations": map[string]interface{}{
			"name":        "included_relations",
			"in":          "query",
			"description": "Comma-separated list of relationships to include in the response",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "author,comments,tags",
		},
		"Fields": map[string]interface{}{
			"name":        "fields",
			"in":          "query",
			"description": "Comma-separated list of fields to include in the response. Reduces payload size.",
			"schema": map[string]interface{}{
				"type": "string",
			},
			"example": "id,name,email,created_at",
		},
	}
}

func CreateCommonResponses() map[string]interface{} {
	return map[string]interface{}{
		"BadRequest": map[string]interface{}{
			"description": "Bad Request - Invalid input parameters or malformed request",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "400",
								"title":  "Bad Request",
								"detail": "The filter parameter contains invalid JSON",
								"source": map[string]string{
									"parameter": "filter",
								},
							},
						},
					},
				},
			},
		},
		"Unauthorized": map[string]interface{}{
			"description": "Unauthorized - Missing or invalid authentication token",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "401",
								"title":  "Unauthorized",
								"detail": "Invalid or expired JWT token",
							},
						},
					},
				},
			},
		},
		"Forbidden": map[string]interface{}{
			"description": "Forbidden - You don't have permission to access this resource",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "403",
								"title":  "Forbidden",
								"detail": "You don't have permission to update this resource",
							},
						},
					},
				},
			},
		},
		"NotFound": map[string]interface{}{
			"description": "Not Found - The requested resource doesn't exist",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "404",
								"title":  "Not Found",
								"detail": "Resource with the specified ID was not found",
							},
						},
					},
				},
			},
		},
		"UnprocessableEntity": map[string]interface{}{
			"description": "Unprocessable Entity - Validation errors",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "422",
								"title":  "Validation Error",
								"detail": "Email must be a valid email address",
								"source": map[string]string{
									"pointer": "/data/attributes/email",
								},
							},
						},
					},
				},
			},
		},
		"TooManyRequests": map[string]interface{}{
			"description": "Too Many Requests - Rate limit exceeded",
			"headers": map[string]interface{}{
				"X-RateLimit-Limit": map[string]interface{}{
					"description": "The maximum number of requests allowed",
					"schema": map[string]interface{}{
						"type": "integer",
					},
				},
				"X-RateLimit-Remaining": map[string]interface{}{
					"description": "The number of requests remaining",
					"schema": map[string]interface{}{
						"type": "integer",
					},
				},
				"X-RateLimit-Reset": map[string]interface{}{
					"description": "Unix timestamp when rate limit resets",
					"schema": map[string]interface{}{
						"type": "integer",
					},
				},
			},
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/RateLimitResponse",
					},
				},
			},
		},
		"InternalServerError": map[string]interface{}{
			"description": "Internal Server Error - Something went wrong on the server",
			"content": map[string]interface{}{
				"application/json": map[string]interface{}{
					"schema": map[string]interface{}{
						"$ref": "#/components/schemas/ErrorResponse",
					},
					"example": map[string]interface{}{
						"errors": []map[string]interface{}{
							{
								"status": "500",
								"title":  "Internal Server Error",
								"detail": "An unexpected error occurred. Please try again later.",
							},
						},
					},
				},
			},
		},
	}
}

func CreateDeleteMethod(tableInfo table_info.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdMethod200Response["description"] = "delete " + tableInfo.TableName + " by reference id"
	deleteByIdMethod200Response["content"] = map[string]interface{}{
		"application/json": map[string]interface{}{
			"schema": map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"meta": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"message": map[string]interface{}{
								"type": "string",
								"example": "Resource deleted successfully",
							},
						},
					},
				},
			},
		},
	}
	deleteByIdResponseMap["204"] = map[string]interface{}{
		"description": "No Content - Resource deleted successfully",
	}
	deleteByIdResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	deleteByIdResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	deleteByIdResponseMap["404"] = map[string]interface{}{
		"$ref": "#/components/responses/NotFound",
	}
	deleteByIdResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
	deleteByIdMethod["responses"] = deleteByIdResponseMap

	deleteByIdMethod["description"] = fmt.Sprintf("Delete a %v", tableInfo.TableName)

	deleteByIdMethod["summary"] = fmt.Sprintf("Delete %v", tableInfo.TableName)
	deleteByIdMethod["tags"] = []string{tableInfo.TableName, "delete"}
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
	deleteByIdMethod["operationId"] = fmt.Sprintf("Delete%s", strcase.ToCamel(tableInfo.TableName))
	return deleteByIdMethod
}

func CreateDeleteRelationMethod(tableInfo table_info.TableInfo) map[string]interface{} {
	deleteByIdMethod := make(map[string]interface{})
	deleteByIdMethod200Response := make(map[string]interface{})
	deleteByIdMethod200Response["description"] = "Successful deletion of relation " + tableInfo.TableName
	deleteBody := make(map[string]interface{})
	deleteBody["type"] = tableInfo.TableName
	deleteByIdMethod["description"] = "Delete a " + tableInfo.TableName

	deleteByIdResponseMap := make(map[string]interface{})
	deleteByIdResponseMap["200"] = deleteByIdMethod200Response
	deleteByIdMethod["responses"] = deleteByIdResponseMap
	deleteByIdMethod["description"] = fmt.Sprintf("Remove a related %v ", tableInfo.TableName)
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
	deleteByIdMethod["requestBody"] = map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"data": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"type": map[string]interface{}{
										"type":    "string",
										"default": tableInfo.TableName,
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
		},
	}

	return deleteByIdMethod
}

func CreatePatchRelationMethod(tableInfo table_info.TableInfo) map[string]interface{} {
	patchByIdMethod := make(map[string]interface{})
	patchByIdMethod200Response := make(map[string]interface{})
	patchByIdMethod200Response["description"] = "Add relation " + tableInfo.TableName
	patchBody := make(map[string]interface{})
	patchBody["type"] = tableInfo.TableName
	patchByIdMethod["description"] = "Patch relation to add " + tableInfo.TableName

	patchByIdResponseMap := make(map[string]interface{})
	patchByIdResponseMap["200"] = patchByIdMethod200Response
	patchByIdMethod["responses"] = patchByIdResponseMap
	patchByIdMethod["description"] = fmt.Sprintf("Patch and add related %v ", tableInfo.TableName)
	patchByIdMethod["tags"] = []string{tableInfo.TableName}
	patchByIdMethod["parameters"] = []map[string]interface{}{
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
	patchByIdMethod["requestBody"] = map[string]interface{}{
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"data": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"type": map[string]interface{}{
									"type":    "string",
									"default": tableInfo.TableName,
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

	return patchByIdMethod
}

func CreateGetMethod(tableInfo table_info.TableInfo, dataInResponse map[string]interface{}) map[string]interface{} {
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
	getByIdResponseMap["401"] = map[string]interface{}{
		"$ref": "#/components/responses/Unauthorized",
	}
	getByIdResponseMap["403"] = map[string]interface{}{
		"$ref": "#/components/responses/Forbidden",
	}
	getByIdResponseMap["404"] = map[string]interface{}{
		"$ref": "#/components/responses/NotFound",
	}
	getByIdResponseMap["429"] = map[string]interface{}{
		"$ref": "#/components/responses/TooManyRequests",
	}
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
	getByIdMethod["operationId"] = fmt.Sprintf("Get%sByReferenceId", strcase.ToCamel(tableInfo.TableName))
	return getByIdMethod
}
func CreatePostRequestExample(tableInfo table_info.TableInfo) map[string]interface{} {
	attributes := make(map[string]interface{})
	for _, col := range tableInfo.Columns {
		if col.IsForeignKey || skipColumns[col.ColumnName] || resource.IsStandardColumn(col.ColumnName) {
			continue
		}
		
		switch col.ColumnType {
		case "email":
			attributes[col.ColumnName] = "user@example.com"
		case "name":
			attributes[col.ColumnName] = "John Doe"
		case "label":
			attributes[col.ColumnName] = "Example Label"
		case "url":
			attributes[col.ColumnName] = "https://example.com"
		case "date":
			attributes[col.ColumnName] = "2024-01-15"
		case "datetime":
			attributes[col.ColumnName] = "2024-01-15T09:30:00Z"
		case "integer":
			attributes[col.ColumnName] = 42
		case "float":
			attributes[col.ColumnName] = 3.14
		case "boolean":
			attributes[col.ColumnName] = true
		case "text":
			attributes[col.ColumnName] = "This is a sample text content"
		default:
			if col.ColumnType == "string" || col.DataType == "varchar" {
				attributes[col.ColumnName] = "example " + col.ColumnName
			}
		}
	}
	
	return map[string]interface{}{
		"data": map[string]interface{}{
			"type":       tableInfo.TableName,
			"attributes": attributes,
		},
	}
}

func CreatePatchMethod(tableInfo table_info.TableInfo) map[string]interface{} {

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

func categorizeAction(actionName string) string {
	switch actionName {
	case "import_files_from_store", "export_data", "export_csv_data", "import_data":
		return "Data Operations"
	case "remove_column", "remove_table", "rename_column", "upload_system_schema", "download_system_schema":
		return "Schema Management"
	case "sync_site_storage", "sync_column_storage", "upload_file", "create_site", "delete_path", "create_folder", "move_path", "list_files", "get_file", "delete_file":
		return "Storage Management"
	case "download_certificate", "download_public_key", "generate_acme_certificate", "generate_self_certificate":
		return "Certificate Management"
	case "signup", "signin", "register_otp", "verify_otp", "send_otp", "reset-password", "reset-password-verify", "oauth_login_begin", "oauth.login.response":
		return "User Management"
	case "restart_daptin", "become_an_administrator", "sync_mail_servers", "install_integration":
		return "System Actions"
	case "generate_random_data", "upload_xls_to_system_schema", "upload_csv_to_system_schema", "add_exchange":
		return "Data Operations"
	default:
		return ""
	}
}

func generateActionDescription(action actionresponse.Action) string {
	descriptions := map[string]string{
		"import_files_from_store": "Imports file data from cloud storage into a specified database table. This action allows you to bulk import files stored in cloud storage services into Daptin's database tables, enabling integration with external file repositories.",
		"install_integration": "Installs and configures a third-party integration. This action sets up external service integrations, enabling Daptin to connect with various APIs, webhooks, and external systems.",
		"download_certificate": "Downloads the SSL/TLS certificate in PEM format for a specific hostname. Use this action to retrieve certificates for backup, inspection, or deployment to other systems.",
		"download_public_key": "Downloads the public key associated with a certificate. This action provides access to the public key component of SSL/TLS certificates for cryptographic operations or verification purposes.",
		"generate_acme_certificate": "Generates a Let's Encrypt SSL/TLS certificate using the ACME protocol. This action automates the process of obtaining free, trusted SSL certificates for your domains.",
		"generate_self_certificate": "Generates a self-signed SSL/TLS certificate. Useful for development environments or internal services where a trusted certificate authority is not required.",
		"register_otp": "Registers a mobile number for OTP-based authentication. This action initiates the process of associating a phone number with a user account for two-factor authentication.",
		"verify_otp": "Verifies an OTP code for authentication. Completes the two-factor authentication process by validating the one-time password sent to the user's registered device.",
		"send_otp": "Sends a one-time password to a registered mobile number or email. Use this action to trigger OTP delivery for authentication or verification purposes.",
		"remove_column": "Permanently removes a column from a database table. This destructive action deletes the specified column and all its data. Use with caution as this operation cannot be undone.",
		"remove_table": "Permanently deletes an entire database table and all its data. This is a destructive operation that removes the table schema and all associated records. Cannot be undone.",
		"rename_column": "Renames a column in a database table. This action updates the column name while preserving all existing data and relationships.",
		"sync_site_storage": "Synchronizes files between a site and its configured cloud storage. This action ensures that site content is properly backed up or distributed across storage providers.",
		"sync_column_storage": "Synchronizes file-type column data with external cloud storage. Ensures that files referenced in database columns are properly stored in the configured cloud storage backend.",
		"sync_mail_servers": "Synchronizes email account configurations with mail servers. Updates mail server connections and ensures email functionality is properly configured.",
		"restart_daptin": "Initiates a system restart to apply configuration changes. This action gracefully restarts the Daptin instance, reloading all configurations and schemas.",
		"generate_random_data": "Generates random test data for a specified table. Useful for testing, development, and demonstration purposes. Automatically creates realistic data based on column types.",
		"export_data": "Exports table data in various formats (JSON, CSV, XML). Supports filtering, column selection, and custom formatting options for data extraction and backup.",
		"export_csv_data": "Exports table data specifically in CSV format. Optimized for spreadsheet applications and data analysis tools.",
		"import_data": "Imports data from various file formats into database tables. Supports JSON, YAML, CSV, Excel, and other formats with options for data validation and transformation.",
		"upload_file": "Uploads a file to configured cloud storage. Handles file transfer to external storage providers with support for path specification and metadata.",
		"create_site": "Creates a new website/application site with specified hosting configuration. Sets up a new site instance with its own storage and routing configuration.",
		"delete_path": "Deletes a file or directory from cloud storage. Removes specified paths from the configured storage backend.",
		"create_folder": "Creates a new directory in cloud storage. Establishes folder structures for organizing files in external storage systems.",
		"move_path": "Moves or renames files/folders in cloud storage. Relocates content within the storage system while preserving file integrity.",
		"list_files": "Lists files and directories at a specified path. Provides directory browsing functionality for site content management.",
		"get_file": "Retrieves a specific file from site storage. Downloads file content for viewing or processing.",
		"delete_file": "Removes a specific file from site storage. Permanently deletes the specified file from the site's storage location.",
		"upload_system_schema": "Uploads and applies a new system configuration schema. Updates Daptin's configuration by uploading JSON, YAML, or other supported schema formats.",
		"download_system_schema": "Downloads the current system configuration as a schema file. Exports the complete Daptin configuration for backup or migration purposes.",
		"become_an_administrator": "Elevates the current user to administrator privileges. Grants full system access - use only in controlled environments or during initial setup.",
		"signup": "Creates a new user account with email and password. Registers a new user in the system with optional email verification.",
		"signin": "Authenticates a user and returns a JWT token. Standard login process that validates credentials and issues an authentication token.",
		"reset-password": "Initiates the password reset process. Sends a verification code to the user's registered email for password recovery.",
		"reset-password-verify": "Completes password reset with verification code. Validates the reset code and sets a new password for the user account.",
		"oauth_login_begin": "Initiates OAuth authentication flow. Redirects to the OAuth provider for third-party authentication.",
		"oauth.login.response": "Handles OAuth provider callback. Processes the OAuth response and creates/updates user account with provider data.",
		"upload_xls_to_system_schema": "Imports Excel data into a database table. Creates or updates table schema based on Excel structure with automatic column mapping.",
		"upload_csv_to_system_schema": "Imports CSV data into a database table. Creates or updates table schema based on CSV structure with automatic column detection.",
		"add_exchange": "Configures a new data exchange for external integrations. Sets up automated data synchronization with external services like Google Sheets.",
	}
	
	if desc, ok := descriptions[action.Name]; ok {
		return desc
	}
	return action.Label
}

func generateActionSchemaDescription(action actionresponse.Action) string {
	baseDesc := fmt.Sprintf("Request schema for the '%s' action. ", action.Label)
	
	if len(action.InFields) == 0 && action.InstanceOptional {
		return baseDesc + "This action requires no input parameters and can be executed without specifying an instance."
	} else if len(action.InFields) == 0 {
		return baseDesc + fmt.Sprintf("This action requires only the reference ID of the %s instance on which to execute it.", action.OnType)
	}
	
	return baseDesc + fmt.Sprintf("This action operates on %s instances and requires the following input parameters to execute successfully.", action.OnType)
}

func getFieldDescription(actionName, fieldName string) (string, bool) {
	fieldDescriptions := map[string]map[string]string{
		"import_files_from_store": {
			"table_name": "The name of the database table where files should be imported. Must be an existing table with appropriate file storage columns.",
		},
		"generate_acme_certificate": {
			"email": "Contact email address for Let's Encrypt notifications. Required for certificate renewal reminders and important notices.",
		},
		"register_otp": {
			"mobile_number": "Mobile phone number to register for OTP delivery. Must be in a valid format with country code (e.g., +1234567890).",
		},
		"verify_otp": {
			"otp": "The one-time password received via SMS or email. Typically a 6-digit numeric code.",
			"mobile_number": "The mobile number where the OTP was sent. Must match the registered number.",
			"email": "The email address associated with the account. Used for account verification.",
		},
		"remove_column": {
			"column_name": "The exact name of the column to remove. This operation is irreversible and will delete all data in this column.",
		},
		"rename_column": {
			"table_name": "The name of the table containing the column to rename.",
			"column_name": "The current name of the column to be renamed.",
			"new_column_name": "The new name for the column. Must follow database naming conventions and not conflict with existing columns.",
		},
		"sync_site_storage": {
			"path": "The directory path to synchronize. Use '/' for root or specify a subdirectory path.",
		},
		"generate_random_data": {
			"count": "Number of random records to generate. Must be a positive integer.",
			"table_name": "Target table for data generation. The table must exist and have a defined schema.",
		},
		"export_data": {
			"table_name": "Name of the table to export data from.",
			"format": "Output format for the export. Supported: json, csv, xml.",
			"columns": "Comma-separated list of column names to include. Leave empty for all columns.",
			"include_headers": "Whether to include column headers in the export (CSV format only).",
		},
		"import_data": {
			"dump_file": "File containing data to import. Supported formats: JSON, YAML, CSV, Excel, TOML, HCL.",
			"truncate_before_insert": "Whether to delete existing data before importing. Use with caution.",
			"batch_size": "Number of records to process in each batch. Larger values are faster but use more memory.",
		},
		"upload_file": {
			"file": "The file to upload to cloud storage. Any file type is supported.",
			"path": "Destination path in cloud storage. Leave empty for root directory.",
		},
		"create_site": {
			"site_type": "Type of site to create (e.g., 'static', 'hugo', 'jekyll').",
			"path": "Storage path where site files will be stored.",
			"hostname": "Domain name or hostname for the site.",
		},
		"signup": {
			"name": "Full name of the user.",
			"email": "Email address for the account. Will be used for login and notifications.",
			"mobile": "Optional mobile number for SMS-based features.",
			"password": "Account password. Must be at least 8 characters.",
			"passwordConfirm": "Password confirmation. Must match the password field.",
		},
		"signin": {
			"email": "Registered email address for the account.",
			"password": "Account password.",
		},
	}
	
	if actionFields, ok := fieldDescriptions[actionName]; ok {
		if desc, ok := actionFields[fieldName]; ok {
			return desc, true
		}
	}
	
	return "", false
}

func generateActionExample(action actionresponse.Action) map[string]interface{} {
	examples := map[string]map[string]interface{}{
		"import_files_from_store": {
			"table_name": "documents",
		},
		"generate_acme_certificate": {
			"email": "admin@example.com",
		},
		"register_otp": {
			"mobile_number": "+1234567890",
		},
		"verify_otp": {
			"otp": "123456",
			"mobile_number": "+1234567890",
			"email": "user@example.com",
		},
		"remove_column": {
			"column_name": "deprecated_field",
		},
		"rename_column": {
			"table_name": "products",
			"column_name": "product_desc",
			"new_column_name": "product_description",
		},
		"generate_random_data": {
			"count": 100,
			"table_name": "test_users",
		},
		"export_data": {
			"table_name": "customers",
			"format": "csv",
			"columns": "name,email,created_at",
			"include_headers": true,
		},
		"signup": {
			"name": "John Doe",
			"email": "john.doe@example.com",
			"mobile": "+1234567890",
			"password": "SecurePass123!",
			"passwordConfirm": "SecurePass123!",
		},
		"signin": {
			"email": "john.doe@example.com",
			"password": "SecurePass123!",
		},
	}
	
	if example, ok := examples[action.Name]; ok {
		if !action.InstanceOptional {
			example[action.OnType+"_id"] = "550e8400-e29b-41d4-a716-446655440000"
		}
		return example
	}
	
	// Generate a basic example if not specifically defined
	basicExample := make(map[string]interface{})
	if !action.InstanceOptional {
		basicExample[action.OnType+"_id"] = "550e8400-e29b-41d4-a716-446655440000"
	}
	
	for _, field := range action.InFields {
		switch field.ColumnType {
		case "email":
			basicExample[field.ColumnName] = "user@example.com"
		case "label", "text":
			basicExample[field.ColumnName] = "example " + field.ColumnName
		case "measurement", "integer":
			basicExample[field.ColumnName] = 10
		case "truefalse", "boolean":
			basicExample[field.ColumnName] = true
		default:
			basicExample[field.ColumnName] = "example_value"
		}
	}
	
	return basicExample
}

func generateActionRequestExample(action actionresponse.Action) map[string]interface{} {
	return generateActionExample(action)
}

func generateActionResponseExample(action actionresponse.Action) []map[string]interface{} {
	// Generate response examples based on action type
	responseExamples := map[string][]map[string]interface{}{
		"signin": {
			{
				"ResponseType": "client.token.set",
				"Attributes": map[string]interface{}{
					"key": "token",
					"value": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
					"expiry": 86400,
				},
			},
		},
		"signup": {
			{
				"ResponseType": "client.notify",
				"Attributes": map[string]interface{}{
					"type": "success",
					"title": "Success",
					"message": "Sign-up successful. Redirecting to sign in",
				},
			},
			{
				"ResponseType": "client.redirect",
				"Attributes": map[string]interface{}{
					"location": "/auth/signin",
					"delay": 2000,
				},
			},
		},
		"download_certificate": {
			{
				"ResponseType": "client.file.download",
				"Attributes": map[string]interface{}{
					"name": "example.com.pem.crt",
					"contentType": "application/x-x509-ca-cert",
					"message": "Certificate for example.com",
				},
			},
		},
		"export_data": {
			{
				"ResponseType": "client.file.download",
				"Attributes": map[string]interface{}{
					"name": "export.csv",
					"contentType": "text/csv",
					"message": "Data export completed",
				},
			},
		},
	}
	
	if examples, ok := responseExamples[action.Name]; ok {
		return examples
	}
	
	// Default response example
	return []map[string]interface{}{
		{
			"ResponseType": "client.notify",
			"Attributes": map[string]interface{}{
				"type": "success",
				"title": "Action Completed",
				"message": fmt.Sprintf("%s action executed successfully", action.Label),
			},
		},
	}
}

func generateCurlExample(action actionresponse.Action) string {
	example := generateActionExample(action)
	exampleJSON, _ := json.Marshal(example)
	
	return fmt.Sprintf(`curl -X POST \\\n  https://your-daptin-instance.com/action/%s/%s \\\n  -H 'Authorization: Bearer YOUR_JWT_TOKEN' \\\n  -H 'Content-Type: application/json' \\\n  -d '%s'`, 
		action.OnType, 
		action.Name, 
		string(exampleJSON))
}
