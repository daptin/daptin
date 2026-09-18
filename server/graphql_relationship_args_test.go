package server

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/resource"
	"github.com/daptin/daptin/server/table_info"
	"github.com/graphql-go/graphql"
)

func TestGraphQLMutationArgumentsIncludeRelationships(t *testing.T) {
	if resource.ColumnManager == nil {
		resource.InitialiseColumnManager()
	}
	categoryRelation := api2go.NewTableRelationWithNames("bbproduct", "product_id", "belongs_to", "bbcategory", "category_id")
	tagsRelation := api2go.NewTableRelationWithNames("bbproduct", "product_id", "has_many_and_belongs_to_many", "bbtag", "tags")
	tagsRelation.Columns = []api2go.ColumnInfo{{Name: "priority", ColumnName: "priority", ColumnType: "integer"}}
	table := table_info.TableInfo{
		TableName: "bbproduct",
		Columns: []api2go.ColumnInfo{
			{Name: "name", ColumnName: "name", ColumnType: "label", IsNullable: false},
			{Name: "category_id", ColumnName: "category_id", ColumnType: "alias", IsForeignKey: true, IsNullable: false,
				ForeignKeyData: api2go.ForeignKeyData{DataSource: "self", Namespace: "bbcategory"}},
			{Name: resource.USER_ACCOUNT_ID_COLUMN, ColumnName: resource.USER_ACCOUNT_ID_COLUMN, ColumnType: "alias", IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{DataSource: "self", Namespace: "user_account"}},
			{Name: "asset", ColumnName: "asset", ColumnType: "file", IsForeignKey: true,
				ForeignKeyData: api2go.ForeignKeyData{DataSource: "cloud_store", Namespace: "files"}},
		},
		Relations: []api2go.TableRelation{categoryRelation, tagsRelation},
	}

	createFields, updateFields := graphqlMutationArguments(table)
	categoryCreate, ok := createFields["category_id"].Type.(*graphql.NonNull)
	if !ok || categoryCreate.OfType != graphql.ID {
		t.Fatalf("required belongs_to create argument should be ID!, got %T %#v", createFields["category_id"].Type, createFields["category_id"].Type)
	}
	if updateFields["category_id"].Type != graphql.ID {
		t.Fatalf("belongs_to update argument should be optional ID, got %#v", updateFields["category_id"].Type)
	}
	if _, exists := createFields["asset"]; exists {
		t.Fatal("cloud-store foreign key was exposed as a resource relationship")
	}
	if _, exists := createFields[resource.USER_ACCOUNT_ID_COLUMN]; exists {
		t.Fatal("server-owned record identity relationship was exposed as caller input")
	}

	tagsList, ok := createFields["tags"].Type.(*graphql.List)
	if !ok {
		t.Fatalf("to-many relationship should be a list, got %T", createFields["tags"].Type)
	}
	itemNonNull, ok := tagsList.OfType.(*graphql.NonNull)
	if !ok {
		t.Fatalf("to-many relationship items should be non-null, got %T", tagsList.OfType)
	}
	item, ok := itemNonNull.OfType.(*graphql.InputObject)
	if !ok {
		t.Fatalf("to-many relationship item should be an input object, got %T", itemNonNull.OfType)
	}
	if _, ok := item.Fields()["reference_id"].Type.(*graphql.NonNull); !ok {
		t.Fatal("relationship reference_id should be required")
	}
	if item.Fields()["priority"] == nil {
		t.Fatal("declared join attribute is missing from relationship input")
	}
}

func TestGraphQLMutationAttributesCreateAndUpdateRelationshipLinkage(t *testing.T) {
	relation := api2go.NewTableRelationWithNames("bbproduct", "product_id", "has_many", "bbtag", "tags")
	relation.Columns = []api2go.ColumnInfo{{Name: "priority", ColumnName: "priority", ColumnType: "integer"}}
	table := table_info.TableInfo{TableName: "bbproduct", Relations: []api2go.TableRelation{relation}}
	args := map[string]interface{}{
		"tags": []interface{}{map[string]interface{}{"reference_id": "tag-reference", "priority": 3}},
	}

	created := graphqlMutationAttributes(table, args, false)["tags"].([]interface{})[0].(map[string]interface{})
	if created["reference_id"] != "tag-reference" {
		t.Fatalf("create linkage lost reference_id: %#v", created)
	}
	if created["attributes"].(map[string]interface{})["priority"] != 3 {
		t.Fatalf("create linkage lost join attributes: %#v", created)
	}

	updated := graphqlMutationAttributes(table, args, true)["tags"].([]interface{})[0].(map[string]interface{})
	if updated["id"] != "tag-reference" {
		t.Fatalf("update linkage did not use resource update shape: %#v", updated)
	}
	if _, exists := updated["reference_id"]; exists {
		t.Fatalf("update linkage retained create-only reference_id: %#v", updated)
	}
}

func TestValidateGraphQLRelationshipAttributesRejectsInvalidReferences(t *testing.T) {
	table := table_info.TableInfo{
		TableName: "bbproduct",
		Columns: []api2go.ColumnInfo{{
			Name: "category_id", ColumnName: "category_id", IsForeignKey: true, IsNullable: false,
			ForeignKeyData: api2go.ForeignKeyData{DataSource: "self", Namespace: "bbcategory"},
		}},
	}
	if err := validateGraphQLRelationshipAttributes(table, map[string]interface{}{"category_id": nil}); err == nil {
		t.Fatal("explicit null was accepted for a required relationship")
	}
	if err := validateGraphQLRelationshipAttributes(table, map[string]interface{}{"category_id": "not-a-reference-id"}); err == nil {
		t.Fatal("malformed relationship reference was accepted")
	}
	if err := validateGraphQLRelationshipAttributes(table, map[string]interface{}{
		"category_id": "019d5df0-a8b7-7c84-9015-d26bb74ae86a",
	}); err != nil {
		t.Fatalf("valid relationship reference was rejected: %v", err)
	}
}
