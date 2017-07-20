package apiblueprint

import (
  "bytes"
  "github.com/artpar/goms/server/resource"
  "github.com/artpar/api2go"
  "fmt"
  "strings"
  "github.com/artpar/goms/server/fakerservice"
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
func (x *BlueprintWriter) Markdown() string {
  return x.buffer.String()
}

var skipColumns = map[string]bool{
  "id":         true,
  "deleted_at": true,
  "permission": true,
  "status":     true,
}

func CreateColumnLine(colInfo api2go.ColumnInfo) string {
  columnType := colInfo.ColumnType

  typ := resource.ColumnManager.GetBlueprintType(columnType)
  if !colInfo.IsNullable && colInfo.DefaultValue == "" && colInfo.ColumnName != "reference_id" {
    typ = typ + ", required"
  }
  return fmt.Sprintf("+ %s (%s) - %s", colInfo.ColumnName, typ, colInfo.ColumnDescription)
}

func BuildApiBlueprint(config *resource.CmsConfig, cruds map[string]*resource.DbResource) string {

  tableMap := map[string]resource.TableInfo{}
  for _, table := range config.Tables {
    tableMap[table.TableName] = table
  }

  blueprintWriter := NewBluePrintWriter()

  blueprintWriter.WriteString("FORMAT: 1A")
  blueprintWriter.WriteString("HOST: http://" + config.Hostname)
  blueprintWriter.WriteString("")

  blueprintWriter.WriteString("# Goms Server API description")
  blueprintWriter.WriteString("Welcome to the **Goms** API. This API provides access to the **Goms** service.")
  blueprintWriter.WriteString("")

  blueprintWriter.WriteString("# Data Structures")
  blueprintWriter.WriteString("")

  blueprintWriter.WriteString("## RelationStructure (object)")
  blueprintWriter.WriteString("+ type (string) - type of the entity of the relations")
  blueprintWriter.WriteString("+ id (string) - id of the entity")
  blueprintWriter.WriteString("")

  for _, tableInfo := range config.Tables {

    if strings.Index(tableInfo.TableName, "_has_") > -1 {
      continue
    }

    blueprintWriter.WriteString("## `" + tableInfo.TableName + "` (object)")
    blueprintWriter.WriteString("")

    for _, colInfo := range tableInfo.Columns {
      if colInfo.IsForeignKey {
        continue
      }
      if skipColumns[colInfo.ColumnName] {
        continue
      }
      blueprintWriter.WriteString(CreateColumnLine(colInfo))
    }

    for _, relation := range tableInfo.Relations {
      if relation.Subject == tableInfo.TableName {
        blueprintWriter.WriteString(CreateForwardRelationLine(relation))
      } else {
        blueprintWriter.WriteString(CreateBackwardRelationLine(relation))
      }
    }

    blueprintWriter.WriteString("")

  }

  for _, tableInfo := range config.Tables {

    if strings.Index(tableInfo.TableName, "_has_") > -1 {
      continue
    }
    fakeObject := fakerservice.NewFakeInstance(tableInfo)

    blueprintWriter.WriteString("# Group " + tableInfo.TableName)
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("Resources in this group are related to " + tableInfo.TableName)
    blueprintWriter.WriteString(fmt.Sprintf("%v has %d relation.", tableInfo.TableName, len(tableInfo.Relations)))
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("## Resource [/api/" + tableInfo.TableName + "?{sort,page%5Bnumber%5D,page%5Bsize%5D,query}]")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Parameters")
    blueprintWriter.WriteString("    + sort (optional, string) - sort results by a columns")
    blueprintWriter.WriteString("    + page%5Bnumber%5D (string, required) - Page number for the query set, starts with 1")
    blueprintWriter.WriteString("    + page%5Bsize%5D (string, required) - Size of one page, try 10")
    blueprintWriter.WriteString("    + query (optional, string) - sort results by a columns")
    blueprintWriter.WriteString("")

    //  BEGIN POST Request
    blueprintWriter.WriteString("### Create a " + tableInfo.TableName + " [POST]")
    blueprintWriter.WriteString(fmt.Sprintf("Create a new %s", tableInfo.TableName))
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Request Json (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("                   {")

    for key, val := range fakeObject {
      blueprintWriter.WriteString(fmt.Sprintf("                        \"%v\": \"%v\",", key, val))
    }
    blueprintWriter.WriteString("                   }")

    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("    + Body")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("            {")

    blueprintWriter.WriteString("               \"data\": {")
    blueprintWriter.WriteString("                   \"attributes\": {")

    for key, val := range fakeObject {
      blueprintWriter.WriteString(fmt.Sprintf("                        \"%v\": \"%v\",", key, val))
    }
    blueprintWriter.WriteString("                   }")
    blueprintWriter.WriteString("                   \"id\": \"" + fakeObject["reference_id"].(string) + "\"")
    blueprintWriter.WriteString("                   \"type\": \"" + tableInfo.TableName + "\"")
    blueprintWriter.WriteString("                   \"relations\": {")

    for _, relation := range tableInfo.Relations {
      if relation.Object == tableInfo.TableName {
        blueprintWriter.WriteString(fmt.Sprintf("                       \"%v\": {", relation.SubjectName))
        blueprintWriter.WriteString(fmt.Sprintf("                           \"data\": {\"id\": \"99719dbe-f3e3-4ef5-acf6-ffc37f189014\", \"type\": \"%v\"}", relation.Subject))
      } else {
        blueprintWriter.WriteString(fmt.Sprintf("                       \"%v\": {", relation.ObjectName))
        blueprintWriter.WriteString(fmt.Sprintf("                           \"data\": {\"id\": \"99719dbe-f3e3-4ef5-acf6-ffc37f189014\", \"type\": \"%v\"}", relation.Object))
      }
      blueprintWriter.WriteString(fmt.Sprintf("                           \"links\": {", ))
      if relation.Object == tableInfo.TableName {
        blueprintWriter.WriteString(fmt.Sprintf("                               \"related\": \"/api/%s/%s/%s\"", tableInfo.TableName, fakeObject["reference_id"], relation.SubjectName))
      } else {
        blueprintWriter.WriteString(fmt.Sprintf("                               \"related\": \"/api/%s/%s/%s\"", tableInfo.TableName, fakeObject["reference_id"], relation.ObjectName))
      }

      blueprintWriter.WriteString(fmt.Sprintf("                               \"self\": \"\""))

      blueprintWriter.WriteString(fmt.Sprintf("                           }", ))
      blueprintWriter.WriteString("                       }")

    }

    blueprintWriter.WriteString("                   }")
    blueprintWriter.WriteString("               }")

    blueprintWriter.WriteString("            }")
    blueprintWriter.WriteString("")

    //  END POST Request
    //  BEGIN GET Request

    blueprintWriter.WriteString("### List all " + tableInfo.TableName + " [GET]")
    blueprintWriter.WriteString("Returns list of " + tableInfo.TableName)
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("    + Body")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString(fmt.Sprintf("    + Attributes (object)"))
    blueprintWriter.WriteString(fmt.Sprintf("            + data - list of queried %s", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("                + attributes (%s) - Attributes of %s", tableInfo.TableName, tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("                + id (string) - reference id of this %s", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("                + relationships - related entities of %v", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("                + type (string) - type of this object"))
    blueprintWriter.WriteString(fmt.Sprintf("            + included - Array of included related entities to %v", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("                + attributes (object) - Attributes of the related entity"))
    blueprintWriter.WriteString(fmt.Sprintf("                + relationships Links to all the relations"))
    for _, relation := range tableInfo.Relations {
      if tableInfo.TableName == relation.Object {

        blueprintWriter.WriteString(fmt.Sprintf("                    + %s (object)", relation.SubjectName))

        if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
          blueprintWriter.WriteString(fmt.Sprintf("                        + data (RelationStructure) - Reference to related %s of %s", relation.SubjectName, relation.Subject, tableInfo.TableName))
        } else {
          blueprintWriter.WriteString(fmt.Sprintf("                        + data (array[RelationStructure]) - References to related %s of %s", relation.SubjectName, relation.Subject, tableInfo.TableName))
        }
      } else {
        blueprintWriter.WriteString(fmt.Sprintf("                    + %s (object)", relation.ObjectName))

        if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
          blueprintWriter.WriteString(fmt.Sprintf("                        + data (RelationStructure) - Reference to related %s of %s", relation.Object, tableInfo.TableName))
        } else {
          blueprintWriter.WriteString(fmt.Sprintf("                        + data (array[RelationStructure]) - References to related %s of %s", relation.Object, tableInfo.TableName))
        }
        blueprintWriter.WriteString(fmt.Sprintf("                        + links (object) - Urls to fetch associated objects"))
        blueprintWriter.WriteString(fmt.Sprintf("                            + related (string) - Urls to Fetch relations of %s", relation.Object))
        blueprintWriter.WriteString(fmt.Sprintf("                            + self (string) - Url to Fetch self %s", relation.Object))
      }
    }

    blueprintWriter.WriteString(fmt.Sprintf("                + type (string) - type of this included object"))
    blueprintWriter.WriteString(fmt.Sprintf("            + links (object)"))
    blueprintWriter.WriteString(fmt.Sprintf("                + current_page (number) - The current page, for pagination"))
    blueprintWriter.WriteString(fmt.Sprintf("                + from (number) - Index of the first records fetched in this result"))
    blueprintWriter.WriteString(fmt.Sprintf("                + last_page (number) - The last page number in current query set"))
    blueprintWriter.WriteString(fmt.Sprintf("                + per_page (number) - This is the number of results in one page"))
    blueprintWriter.WriteString(fmt.Sprintf("                + to (number) - Index of the last record feched in this result"))
    blueprintWriter.WriteString(fmt.Sprintf("                + total (number) - Total number of records"))
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("## Resource [/api/" + tableInfo.TableName + "/{referenceId}]")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Parameters")
    blueprintWriter.WriteString("    + referenceId (required, string) - reference id of the " + tableInfo.TableName + " to be fetched")
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("### Get a single " + tableInfo.TableName + " by reference id [GET]")
    blueprintWriter.WriteString("Returns the " + tableInfo.TableName)
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString(fmt.Sprintf("    + Attributes (object)"))
    blueprintWriter.WriteString(fmt.Sprintf("        + data - list of queried %s", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("            + attributes (%s) - Attributes of %s", tableInfo.TableName, tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("            + id (string) - reference id of this %s", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("            + relationships - related entities of %v", tableInfo.TableName))

    for _, relation := range tableInfo.Relations {
      if tableInfo.TableName == relation.Object {

        blueprintWriter.WriteString(fmt.Sprintf("                + %s (object)", relation.SubjectName))

        if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
          blueprintWriter.WriteString(fmt.Sprintf("                    + data (RelationStructure) - Reference to related %s of %s", relation.SubjectName, relation.Subject, tableInfo.TableName))
        } else {
          blueprintWriter.WriteString(fmt.Sprintf("                    + data (array[RelationStructure]) - References to related %s of %s", relation.SubjectName, relation.Subject, tableInfo.TableName))
        }
      } else {
        blueprintWriter.WriteString(fmt.Sprintf("                + %s (object)", relation.ObjectName))

        if relation.Relation == "belongs_to" || relation.Relation == "has_one" {
          blueprintWriter.WriteString(fmt.Sprintf("                    + data (RelationStructure) - Reference to related %s of %s", relation.Object, tableInfo.TableName))
        } else {
          blueprintWriter.WriteString(fmt.Sprintf("                    + data (array[RelationStructure]) - References to related %s of %s", relation.Object, tableInfo.TableName))
        }
        blueprintWriter.WriteString(fmt.Sprintf("                    + links (object) - Urls to fetch associated objects"))
        blueprintWriter.WriteString(fmt.Sprintf("                        + related (string) - Urls to Fetch relations of %s", relation.Object))
        blueprintWriter.WriteString(fmt.Sprintf("                        + self (string) - Url to Fetch self %s", relation.Object))
      }
    }

    blueprintWriter.WriteString(fmt.Sprintf("            + type (string) - this is the type name returned along with each object, will be %s here", tableInfo.TableName))
    blueprintWriter.WriteString(fmt.Sprintf("        + included"))
    blueprintWriter.WriteString(fmt.Sprintf("            + attributes (object)"))
    blueprintWriter.WriteString(fmt.Sprintf("            + relationships"))
    blueprintWriter.WriteString(fmt.Sprintf("            + type (string)"))
    blueprintWriter.WriteString(fmt.Sprintf("        + links (object)"))
    blueprintWriter.WriteString(fmt.Sprintf("            + current_page (number) - The current page, for pagination"))
    blueprintWriter.WriteString(fmt.Sprintf("            + from (number) - Index of the first records fetched in this result"))
    blueprintWriter.WriteString(fmt.Sprintf("            + last_page (number) - The last page number in current query set"))
    blueprintWriter.WriteString(fmt.Sprintf("            + per_page (number) - This is the number of results in one page"))
    blueprintWriter.WriteString(fmt.Sprintf("            + to (number) - Index of the last record feched in this result"))
    blueprintWriter.WriteString(fmt.Sprintf("            + total (number) - Total number of records"))
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("### Patch instance of " + tableInfo.TableName + " [PATCH]")
    blueprintWriter.WriteString(fmt.Sprintf("Edit existing %s", tableInfo.TableName))
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Request Json (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("                   {")

    for key, val := range fakeObject {
      blueprintWriter.WriteString(fmt.Sprintf("                        \"%v\": \"%v\",", key, val))
    }
    blueprintWriter.WriteString("                   }")

    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("    + Body")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("            {")

    blueprintWriter.WriteString("               \"data\": {")
    blueprintWriter.WriteString("                   \"attributes\": {")

    for key, val := range fakeObject {
      blueprintWriter.WriteString(fmt.Sprintf("                        \"%v\": \"%v\",", key, val))
    }
    blueprintWriter.WriteString("                   }")
    blueprintWriter.WriteString("                   \"id\": \"" + fakeObject["reference_id"].(string) + "\"")
    blueprintWriter.WriteString("                   \"type\": \"" + tableInfo.TableName + "\"")
    blueprintWriter.WriteString("                   \"relations\": {")

    for _, relation := range tableInfo.Relations {
      if relation.Object == tableInfo.TableName {
        blueprintWriter.WriteString(fmt.Sprintf("                       \"%v\": {", relation.SubjectName))
        blueprintWriter.WriteString(fmt.Sprintf("                           \"data\": {\"id\": \"99719dbe-f3e3-4ef5-acf6-ffc37f189014\", \"type\": \"%v\"}", relation.Subject))
      } else {
        blueprintWriter.WriteString(fmt.Sprintf("                       \"%v\": {", relation.ObjectName))
        blueprintWriter.WriteString(fmt.Sprintf("                           \"data\": {\"id\": \"99719dbe-f3e3-4ef5-acf6-ffc37f189014\", \"type\": \"%v\"}", relation.Object))
      }
      blueprintWriter.WriteString(fmt.Sprintf("                           \"links\": {", ))
      if relation.Object == tableInfo.TableName {
        blueprintWriter.WriteString(fmt.Sprintf("                               \"related\": \"/api/%s/%s/%s\"", tableInfo.TableName, fakeObject["reference_id"], relation.SubjectName))
      } else {
        blueprintWriter.WriteString(fmt.Sprintf("                               \"related\": \"/api/%s/%s/%s\"", tableInfo.TableName, fakeObject["reference_id"], relation.ObjectName))
      }

      blueprintWriter.WriteString(fmt.Sprintf("                               \"self\": \"\""))

      blueprintWriter.WriteString(fmt.Sprintf("                           }", ))
      blueprintWriter.WriteString("                       }")

    }

    blueprintWriter.WriteString("                   }")
    blueprintWriter.WriteString("               }")

    blueprintWriter.WriteString("            }")
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("### Delete an instance of " + tableInfo.TableName + " [DELETE]")
    blueprintWriter.WriteString(fmt.Sprintf("Delete an existing %s", tableInfo.TableName))
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Request Json (application/vnd.api+json)")
    blueprintWriter.WriteString("")

    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("    + Body")
    blueprintWriter.WriteString("")
    blueprintWriter.WriteString("            {}")
    blueprintWriter.WriteString("")

    for _, relation := range tableInfo.Relations {

      var name, typeName string
      if relation.Object == tableInfo.TableName {
        name = relation.SubjectName
        typeName = relation.Subject
      } else {
        name = relation.ObjectName
        typeName = relation.Object
      }

      blueprintWriter.WriteString("## Resource [/api/" + tableInfo.TableName + "/{referenceId}/" + name)
      blueprintWriter.WriteString(typeName + " associated with " + tableInfo.TableName)
      blueprintWriter.WriteString("")
      blueprintWriter.WriteString("+ Parameters")
      blueprintWriter.WriteString("    + sort (optional, string) - sort results by a columns")
      blueprintWriter.WriteString("    + page%5Bnumber%5D (string, required) - Page number for the query set, starts with 1")
      blueprintWriter.WriteString("    + page%5Bsize%5D (string, required) - Size of one page, try 10")
      blueprintWriter.WriteString("    + query (optional, string) - sort results by a columns")
      blueprintWriter.WriteString("    + referenceId (string, required) - reference id of the parent object as path param")
      blueprintWriter.WriteString("")

      blueprintWriter.WriteString("## Fetch related " + typeName + " which are of type " + name + " [GET]")
      blueprintWriter.WriteString(fmt.Sprintf("Fetch related %s", typeName))
      blueprintWriter.WriteString("")

      blueprintWriter.WriteString("+ Response 200 (application/vnd.api+json)")
      blueprintWriter.WriteString("")
      blueprintWriter.WriteString("    + Body")
      blueprintWriter.WriteString("")
      blueprintWriter.WriteString(fmt.Sprintf("    + Attributes (object)"))
      blueprintWriter.WriteString(fmt.Sprintf("            + data - list of queried %s", typeName))
      blueprintWriter.WriteString(fmt.Sprintf("                + attributes (%s) - Attributes of %s", typeName, name))
      blueprintWriter.WriteString(fmt.Sprintf("                + id (string) - reference id of this %s", typeName))
      blueprintWriter.WriteString(fmt.Sprintf("                + relationships - related entities of %v", typeName))
      blueprintWriter.WriteString(fmt.Sprintf("                + type (string) - type of this object"))
      blueprintWriter.WriteString(fmt.Sprintf("            + included - Array of included related entities to %v", typeName))
      blueprintWriter.WriteString(fmt.Sprintf("                + attributes (object) - Attributes of the related entity"))
      blueprintWriter.WriteString(fmt.Sprintf("                + relationships Links to all the relations"))

      subRelations := tableMap[typeName].Relations
      for _, subRelation := range subRelations {
        if tableInfo.TableName == subRelation.Object {

          blueprintWriter.WriteString(fmt.Sprintf("                    + %s (object)", subRelation.SubjectName))

          if subRelation.Relation == "belongs_to" || subRelation.Relation == "has_one" {
            blueprintWriter.WriteString(fmt.Sprintf("                        + data (RelationStructure) - Reference to related %s of %s", subRelation.SubjectName, subRelation.Subject, typeName))
          } else {
            blueprintWriter.WriteString(fmt.Sprintf("                        + data (array[RelationStructure]) - References to related %s of %s", subRelation.SubjectName, subRelation.Subject, typeName))
          }
        } else {
          blueprintWriter.WriteString(fmt.Sprintf("                    + %s (object)", subRelation.ObjectName))

          if subRelation.Relation == "belongs_to" || subRelation.Relation == "has_one" {
            blueprintWriter.WriteString(fmt.Sprintf("                        + data (RelationStructure) - Reference to related %s of %s", subRelation.Object, typeName))
          } else {
            blueprintWriter.WriteString(fmt.Sprintf("                        + data (array[RelationStructure]) - References to related %s of %s", subRelation.Object, typeName))
          }
          blueprintWriter.WriteString(fmt.Sprintf("                        + links (object) - Urls to fetch associated objects"))
          blueprintWriter.WriteString(fmt.Sprintf("                            + related (string) - Urls to Fetch relations of %s", subRelation.Object))
          blueprintWriter.WriteString(fmt.Sprintf("                            + self (string) - Url to Fetch self %s", subRelation.Object))
        }
      }

      blueprintWriter.WriteString(fmt.Sprintf("                + type (string) - type of this included object"))
      blueprintWriter.WriteString(fmt.Sprintf("            + links (object)"))
      blueprintWriter.WriteString(fmt.Sprintf("                + current_page (number) - The current page, for pagination"))
      blueprintWriter.WriteString(fmt.Sprintf("                + from (number) - Index of the first records fetched in this result"))
      blueprintWriter.WriteString(fmt.Sprintf("                + last_page (number) - The last page number in current query set"))
      blueprintWriter.WriteString(fmt.Sprintf("                + per_page (number) - This is the number of results in one page"))
      blueprintWriter.WriteString(fmt.Sprintf("                + to (number) - Index of the last record feched in this result"))
      blueprintWriter.WriteString(fmt.Sprintf("                + total (number) - Total number of records"))
      blueprintWriter.WriteString("")

    }

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

  return fmt.Sprintf("+ %s (%s) - %s", relation.GetObjectName(), otherObjectName, relationDescription)
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

  return fmt.Sprintf("+ %s (%s) - %s", relation.GetSubjectName(), otherObjectName, relationDescription)
}
