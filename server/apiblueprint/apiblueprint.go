package apiblueprint

import (
	"bytes"
	"github.com/artpar/goms/server/resource"
	"github.com/artpar/api2go"
	"fmt"
	"strings"
	//"github.com/artpar/goms/server/fakerservice"
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

func CreateColumnLine(colInfo api2go.ColumnInfo, blueprintWriter *BlueprintWriter) {
	columnType := colInfo.ColumnType

	typ := resource.ColumnManager.GetBlueprintType(columnType)
	blueprintWriter.WriteStringf("      %s:", colInfo.ColumnName)
	blueprintWriter.WriteStringf("        type: %s", typ)
	blueprintWriter.WriteStringf("        required: %v", colInfo.IsNullable)
}

func BuildApiBlueprint(config *resource.CmsConfig, cruds map[string]*resource.DbResource) string {

	tableMap := map[string]resource.TableInfo{}
	for _, table := range config.Tables {
		tableMap[table.TableName] = table
	}

	parent := make(map[string]interface{})

	blueprintWriter := NewBluePrintWriter()

	blueprintWriter.WriteString("#%RAML 1.0")
	blueprintWriter.WriteString("")

	parent["title"] = "Goms server"
	parent["version"] = "v1"
	parent["baseUri"] = fmt.Sprintf("http://%v", config.Hostname)
	parent["mediaType"] = "application/json"
	parent["protocols"] = []string{"HTTP", "HTTPS"}
	parent["description"] = "Goms server RAML Spec"

	dataTypesMap := make(map[string]interface{})
	parent["types"] = dataTypesMap
	blueprintWriter.WriteString("title: Goms Server API description")
	blueprintWriter.WriteString("version: v1")
	blueprintWriter.WriteString("baseUri: http://" + config.Hostname)
	blueprintWriter.WriteString("mediaType:")
	blueprintWriter.WriteString("protocols: [ HTTP, HTTPS ]")
	blueprintWriter.WriteString("description: Welcome to the GoMS")

	blueprintWriter.WriteString("types:")
	blueprintWriter.WriteString("  RelatedStructure:")
	blueprintWriter.WriteString("    type: object")
	blueprintWriter.WriteString("    properties:")
	blueprintWriter.WriteString("      id: ")
	blueprintWriter.WriteString("        type: string")
	blueprintWriter.WriteString("        description: Id of the included object")
	blueprintWriter.WriteString("      type: ")
	blueprintWriter.WriteString("        type: string")
	blueprintWriter.WriteString("        description: type of the included object")
	blueprintWriter.WriteString("  Pagination:")
	blueprintWriter.WriteString("    type: object")
	blueprintWriter.WriteString("    properties:")
	blueprintWriter.WriteString("      page[number]: number")
	blueprintWriter.WriteString("      page[limit]: number")
	blueprintWriter.WriteString("  PaginationStatus:")
	blueprintWriter.WriteString("    type: object")
	blueprintWriter.WriteString("    properties: ")
	blueprintWriter.WriteString("      current_page:")
	blueprintWriter.WriteString("        type: number")
	blueprintWriter.WriteString("        description: The current page, for pagination")
	blueprintWriter.WriteString("      from:")
	blueprintWriter.WriteString("        type: number")
	blueprintWriter.WriteString("        description: From page")
	blueprintWriter.WriteString("      last_page:")
	blueprintWriter.WriteString("        type: number")
	blueprintWriter.WriteString("        description: The last page number in current query set")
	blueprintWriter.WriteString("      per_page:")
	blueprintWriter.WriteString("        type: number")
	blueprintWriter.WriteString("        description: This is the number of results in one page")
	blueprintWriter.WriteString("      to:")
	blueprintWriter.WriteString("        type: number")
	blueprintWriter.WriteString("        description: Index of the last record feched in this result")
	blueprintWriter.WriteString("      total:")
	blueprintWriter.WriteString("        type: number")
	blueprintWriter.WriteString("        description: Total number of records")
	blueprintWriter.WriteString("  IncludedRelationship:")
	blueprintWriter.WriteString("    type: object")
	blueprintWriter.WriteString("    properties: ")
	blueprintWriter.WriteString("      data:")
	blueprintWriter.WriteString("        type: array")
	blueprintWriter.WriteString("        items: object")
	blueprintWriter.WriteString("      links:")
	blueprintWriter.WriteString("        type: object")
	blueprintWriter.WriteString("        properties:")
	blueprintWriter.WriteString("          related:")
	blueprintWriter.WriteString("            type: string")
	blueprintWriter.WriteString("          self:")
	blueprintWriter.WriteString("            type: string")
	blueprintWriter.WriteString("  ReferenceToIncludedObject:")
	blueprintWriter.WriteString("    type: object")
	blueprintWriter.WriteString("    properties: ")
	blueprintWriter.WriteString("      data: ")
	blueprintWriter.WriteString("        type: RelatedStructure")
	blueprintWriter.WriteString("        description: Associated objects which are also included in the current response")
	blueprintWriter.WriteString("      links:")
	blueprintWriter.WriteString("        type: object")
	blueprintWriter.WriteString("        properties: ")
	blueprintWriter.WriteString("          related: ")
	blueprintWriter.WriteString("            type: string")
	blueprintWriter.WriteString("            description: link to related objects")
	blueprintWriter.WriteString("          self: ")
	blueprintWriter.WriteString("            type: string")
	blueprintWriter.WriteString("            description: link to self")

	for _, tableInfo := range config.Tables {

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

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
			CreateColumnLine(colInfo, &blueprintWriter)
		}

		for _, relation := range tableInfo.Relations {
			if relation.Subject == tableInfo.TableName {
				blueprintWriter.WriteString(CreateForwardRelationLine(relation))
			} else {
				blueprintWriter.WriteString(CreateBackwardRelationLine(relation))
			}
		}

	}

	for _, tableInfo := range config.Tables {

		// skip join tables
		if strings.Index(tableInfo.TableName, "_has_") > -1 {
			continue
		}

		//fakeObject := fakerservice.NewFakeInstance(tableInfo)

		blueprintWriter.WriteString("/api/" + tableInfo.TableName + ":")
		blueprintWriter.WriteString("  displayName: " + tableInfo.TableName)
		blueprintWriter.WriteString("  description: |")
		blueprintWriter.WriteString("    Resources in this group are related to " + tableInfo.TableName)
		blueprintWriter.WriteString("    " + fmt.Sprintf("%v has %d relation.", tableInfo.TableName, len(tableInfo.Relations)))

		//  BEGIN POST Request
		blueprintWriter.WriteString("  post:")
		blueprintWriter.WriteStringf("    displayName: New %s", tableInfo.TableName)
		blueprintWriter.WriteStringf("    description: Create a new %s", tableInfo.TableName)
		blueprintWriter.WriteString("    body: ")
		blueprintWriter.WriteString("      type: " + tableInfo.TableName)
		blueprintWriter.WriteString("    responses: ")
		blueprintWriter.WriteString("      200: ")
		blueprintWriter.WriteString("        body:")
		blueprintWriter.WriteString("          type: object")
		blueprintWriter.WriteString("          properties:")
		blueprintWriter.WriteString("            data:")
		blueprintWriter.WriteString("              type: object")
		blueprintWriter.WriteString("              properties: ")
		blueprintWriter.WriteString("                attributes: " + tableInfo.TableName)
		blueprintWriter.WriteString("                id: ")
		blueprintWriter.WriteString("                  type: string")
		blueprintWriter.WriteString("                type: ")
		blueprintWriter.WriteString("                  type: string")
		blueprintWriter.WriteString("                relationships:")

		for _, relation := range tableInfo.Relations {
			if relation.Object == tableInfo.TableName {
				blueprintWriter.WriteString(fmt.Sprintf("                  %v: ReferenceToIncludedObject", relation.SubjectName))
			} else {
				blueprintWriter.WriteString(fmt.Sprintf("                  %v: ReferenceToIncludedObject", relation.ObjectName))
			}
		}

		//  END POST Request

		//  BEGIN GET Request

		blueprintWriter.WriteString("  get:")
		blueprintWriter.WriteString("    description: Returns list of " + tableInfo.TableName)
		blueprintWriter.WriteString("    displayName: Get " + tableInfo.TableName)
		blueprintWriter.WriteString("    uriParameters:")
		blueprintWriter.WriteString("      sort: ")
		blueprintWriter.WriteString("        type: string")
		blueprintWriter.WriteString("        required: false")
		blueprintWriter.WriteString("        description: field name to sort by")
		blueprintWriter.WriteString("      page%5Bnumber%5D:")
		blueprintWriter.WriteString("        type: number")
		blueprintWriter.WriteString("        required: false")
		blueprintWriter.WriteString("        description: Page number for the query set, starts with 1")
		blueprintWriter.WriteString("      page%5Bsize%5D:")
		blueprintWriter.WriteString("        type: number")
		blueprintWriter.WriteString("        required: false")
		blueprintWriter.WriteString("        description: Size of one page, try 10")
		blueprintWriter.WriteString("      query: ")
		blueprintWriter.WriteString("       type: string")
		blueprintWriter.WriteString("       required: false")
		blueprintWriter.WriteString("       description: search text in indexed columns")
		blueprintWriter.WriteString("    responses:")
		blueprintWriter.WriteString("      200: ")
		blueprintWriter.WriteString("        body: ")
		blueprintWriter.WriteString("          type: object")
		blueprintWriter.WriteString("          properties: ")
		blueprintWriter.WriteString(fmt.Sprintf("          data: "))
		blueprintWriter.WriteString(fmt.Sprintf("            attributes: "))
		blueprintWriter.WriteString(fmt.Sprintf("              type: object"))
		blueprintWriter.WriteString(fmt.Sprintf("              description: Attributes of %s", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("            id: "))
		blueprintWriter.WriteString(fmt.Sprintf("              type: string"))
		blueprintWriter.WriteString(fmt.Sprintf("              description: reference id of the %s", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("            relationships: ", ))
		blueprintWriter.WriteString(fmt.Sprintf("              description: - related entities of %v", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("            type: "))
		blueprintWriter.WriteString(fmt.Sprintf("              type: string"))
		blueprintWriter.WriteString(fmt.Sprintf("              description: type of this object"))

		blueprintWriter.WriteString(fmt.Sprintf("              + type (string) - type of this included object"))
		blueprintWriter.WriteString(fmt.Sprintf("            links: "))
		blueprintWriter.WriteString(fmt.Sprintf("              type: PaginationStatus"))

		blueprintWriter.WriteString("  /{referenceId}:")
		blueprintWriter.WriteString("    uriParameters")
		blueprintWriter.WriteString("      referenceId: ")
		blueprintWriter.WriteString("        type: string")
		blueprintWriter.WriteString("        description: reference id of the " + tableInfo.TableName + " to be fetched")
		blueprintWriter.WriteString("        required: true")
		blueprintWriter.WriteString("    get:")
		blueprintWriter.WriteString("      description: Get a single " + tableInfo.TableName + " by reference id")
		blueprintWriter.WriteString("      displayName: Returns the " + tableInfo.TableName)

		blueprintWriter.WriteString("      responses:")
		blueprintWriter.WriteString("        200:")
		blueprintWriter.WriteString("          body:")
		blueprintWriter.WriteString(fmt.Sprintf("          data:"))
		blueprintWriter.WriteString(fmt.Sprintf("            type: object"))
		blueprintWriter.WriteString(fmt.Sprintf("            description: list of queried %s", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("            properties:"))
		blueprintWriter.WriteString(fmt.Sprintf("              attributes:"))
		blueprintWriter.WriteString(fmt.Sprintf("                type: object"))
		blueprintWriter.WriteString(fmt.Sprintf("                description: Attributes of %s", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("              id: "))
		blueprintWriter.WriteString(fmt.Sprintf("                type: string"))
		blueprintWriter.WriteString(fmt.Sprintf("                description: - reference id of this %s", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("              relationships:"))
		blueprintWriter.WriteString(fmt.Sprintf("                description:  - related entities of %v", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("                type: object"))
		blueprintWriter.WriteString(fmt.Sprintf("                properties:"))

		for _, relation := range tableInfo.Relations {
			if tableInfo.TableName == relation.Object {

				blueprintWriter.WriteString(fmt.Sprintf("                  %s: ", relation.SubjectName))

				if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
					blueprintWriter.WriteString(fmt.Sprintf("                    type: ReferenceToIncludedObject"))
					blueprintWriter.WriteString(fmt.Sprintf("                    description: Reference to related %s of %s", relation.SubjectName, tableInfo.TableName))
				} else {
					blueprintWriter.WriteString(fmt.Sprintf("                    type: ReferenceToIncludedObject[]"))
					blueprintWriter.WriteString(fmt.Sprintf("                    description: References to related %s of %s", relation.SubjectName, tableInfo.TableName))
				}
			} else {
				blueprintWriter.WriteString(fmt.Sprintf("                  %s: ", relation.ObjectName))

				if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
					blueprintWriter.WriteString(fmt.Sprintf("                data:"))
					blueprintWriter.WriteString(fmt.Sprintf("                  type: ReferenceToIncludedObject"))
					blueprintWriter.WriteString(fmt.Sprintf("                  description: Reference to related %s of %s", relation.Object, tableInfo.TableName))
				} else {
					blueprintWriter.WriteString(fmt.Sprintf("                data:"))
					blueprintWriter.WriteString(fmt.Sprintf("                  type: ReferenceToIncludedObject[]"))
					blueprintWriter.WriteString(fmt.Sprintf("                  description: References to related %s of %s", relation.Object, tableInfo.TableName))
				}
				blueprintWriter.WriteString(fmt.Sprintf("                links:"))
				blueprintWriter.WriteString(fmt.Sprintf("                  type: object"))
				blueprintWriter.WriteString(fmt.Sprintf("                  description: Urls to fetch associated objects"))
				blueprintWriter.WriteString(fmt.Sprintf("                  properties:"))
				blueprintWriter.WriteString(fmt.Sprintf("                    related:"))
				blueprintWriter.WriteString(fmt.Sprintf("                      type: string"))
				blueprintWriter.WriteString(fmt.Sprintf("                      descriptionUrls to Fetch relations of %s", relation.Object))
				blueprintWriter.WriteString(fmt.Sprintf("                    self:"))
				blueprintWriter.WriteString(fmt.Sprintf("                      type: string"))
				blueprintWriter.WriteString(fmt.Sprintf("                      description: Url to Fetch self %s", relation.Object))
			}
		}

		blueprintWriter.WriteString(fmt.Sprintf("              type:"))
		blueprintWriter.WriteString(fmt.Sprintf("                type: string"))
		blueprintWriter.WriteString(fmt.Sprintf("                description: this is the type name returned along with each object, will be %s here", tableInfo.TableName))
		blueprintWriter.WriteString(fmt.Sprintf("            included:"))
		blueprintWriter.WriteString(fmt.Sprintf("              type: array"))
		blueprintWriter.WriteString(fmt.Sprintf("              items: "))
		blueprintWriter.WriteString(fmt.Sprintf("                type: object"))
		blueprintWriter.WriteString(fmt.Sprintf("              relationships:"))
		blueprintWriter.WriteString(fmt.Sprintf("                type:"))
		blueprintWriter.WriteString(fmt.Sprintf("                  type: array"))
		blueprintWriter.WriteString(fmt.Sprintf("                  items:"))
		blueprintWriter.WriteString(fmt.Sprintf("                    type: object"))
		blueprintWriter.WriteString(fmt.Sprintf("                  description: Type of the related object"))
		blueprintWriter.WriteString(fmt.Sprintf("            links:"))
		blueprintWriter.WriteString(fmt.Sprintf("              type: PaginationStatus"))
		blueprintWriter.WriteString("")

		blueprintWriter.WriteString("    patch:")
		blueprintWriter.WriteString(fmt.Sprintf("    description: Edit existing %s", tableInfo.TableName))
		blueprintWriter.WriteString("      body: ")
		blueprintWriter.WriteString("      type: " + tableInfo.TableName)
		blueprintWriter.WriteString("        responses: ")
		blueprintWriter.WriteString("          200: ")
		blueprintWriter.WriteString("            body: ")
		blueprintWriter.WriteString("              type: object")
		blueprintWriter.WriteString("              properties:")
		blueprintWriter.WriteString("                data:")
		blueprintWriter.WriteString("                  type: object")
		blueprintWriter.WriteString("                    properties:")
		blueprintWriter.WriteString("                      attributes: " + tableInfo.TableName)
		blueprintWriter.WriteString("                      id: ")
		blueprintWriter.WriteString("                        type: string")
		blueprintWriter.WriteString("                      type: ")
		blueprintWriter.WriteString("                        type: string")
		blueprintWriter.WriteString("                      relations:")

		for _, relation := range tableInfo.Relations {
			if relation.Object == tableInfo.TableName {
				blueprintWriter.WriteString(fmt.Sprintf("                        %v: ReferenceToIncludedObject", relation.SubjectName))
			} else {
				blueprintWriter.WriteString(fmt.Sprintf("                        %v: ReferenceToIncludedObject", relation.ObjectName))
			}
		}

		blueprintWriter.WriteString("    delete:")
		blueprintWriter.WriteString(fmt.Sprintf("      description: Delete an existing %s", tableInfo.TableName))

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

	return blueprintWriter.Markdown()

}

func CreateForwardRelationLine(relation api2go.TableRelation) string {
	relationDescription := relation.GetRelation()

	otherObjectName := relation.GetObject()
	switch relationDescription {
	case "has_one":
		relationDescription = "Has one " + otherObjectName
	case "has_many":
		relationDescription = "Has many " + otherObjectName
	case "belongs_to":
		relationDescription = "Belongs to " + otherObjectName
	case "has_many_and_belongs_to_many":
		relationDescription = "Has many and belongs to " + otherObjectName
	}

	return fmt.Sprintf("      %s: %s", relation.GetObjectName(), otherObjectName)
}

func CreateBackwardRelationLine(relation api2go.TableRelation) string {
	relationDescription := relation.GetRelation()

	otherObjectName := relation.GetSubject()
	switch relationDescription {
	case "has_one":
		relationDescription = "Has one " + otherObjectName
	case "has_many":
		relationDescription = "Has many " + otherObjectName
	case "belongs_to":
		relationDescription = "Belongs to " + otherObjectName
	case "has_many_and_belongs_to_many":
		relationDescription = "Has many and belongs to " + otherObjectName
	}

	return fmt.Sprintf("      %s: %s", relation.GetSubjectName(), otherObjectName)
}
