package resource

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
)

func TestCheckRelationsMergesGeneratedJoinTableIntoDeclaredTable(t *testing.T) {
	const joinTableName = "world_world_id_has_usergroup_usergroup_id"

	config := CmsConfig{
		Tables: []table_info.TableInfo{
			{TableName: "world"},
			{TableName: "usergroup"},
			{
				TableName:         joinTableName,
				DefaultPermission: auth.AuthPermission(638976),
			},
		},
	}

	CheckRelations(&config)

	var joinTable table_info.TableInfo
	count := 0
	for _, table := range config.Tables {
		if table.TableName != joinTableName {
			continue
		}
		count++
		joinTable = table
	}

	if count != 1 {
		t.Fatalf("expected one %s table, got %d", joinTableName, count)
	}
	if joinTable.DefaultPermission != auth.AuthPermission(638976) {
		t.Fatalf("expected declared default permission 638976, got %d", joinTable.DefaultPermission)
	}
	if !joinTable.IsJoinTable {
		t.Fatalf("expected %s to be marked as a join table", joinTableName)
	}
	if joinTable.IsTopLevel {
		t.Fatalf("expected %s to stay non-top-level", joinTableName)
	}
	if !tableHasColumn(joinTable, "world_id") {
		t.Fatalf("expected generated world_id column on %s", joinTableName)
	}
	if !tableHasColumn(joinTable, "usergroup_id") {
		t.Fatalf("expected generated usergroup_id column on %s", joinTableName)
	}
}

func tableHasColumn(table table_info.TableInfo, columnName string) bool {
	for _, column := range table.Columns {
		if column.ColumnName == columnName {
			return true
		}
	}
	return false
}
