package server

import (
	"testing"

	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
)

func TestMergeTablesSyncsYamlPermissionsForExistingTable(t *testing.T) {
	existingTables := []table_info.TableInfo{
		{
			TableName:         "contract",
			Permission:        auth.AuthPermission(16256),
			DefaultPermission: auth.AuthPermission(16256),
		},
	}
	initConfigTables := []table_info.TableInfo{
		{
			TableName:         "contract",
			DefaultPermission: auth.AuthPermission(16257),
		},
	}

	merged := MergeTables(existingTables, initConfigTables)

	if len(merged) != 1 {
		t.Fatalf("expected one merged table, got %d", len(merged))
	}
	if merged[0].DefaultPermission != auth.AuthPermission(16257) {
		t.Fatalf("expected default permission to sync from YAML, got %d", merged[0].DefaultPermission)
	}
	if merged[0].Permission != auth.AuthPermission(16256) {
		t.Fatalf("expected table permission to preserve database value when Permission is omitted, got %d", merged[0].Permission)
	}
}

func TestMergeTablesSyncsExplicitYamlTablePermissionForExistingTable(t *testing.T) {
	existingTables := []table_info.TableInfo{
		{
			TableName:         "contract",
			Permission:        auth.AuthPermission(16256),
			DefaultPermission: auth.AuthPermission(16256),
		},
	}
	initConfigTables := []table_info.TableInfo{
		{
			TableName:         "contract",
			Permission:        auth.AuthPermission(16258),
			DefaultPermission: auth.AuthPermission(16257),
		},
	}

	merged := MergeTables(existingTables, initConfigTables)

	if len(merged) != 1 {
		t.Fatalf("expected one merged table, got %d", len(merged))
	}
	if merged[0].DefaultPermission != auth.AuthPermission(16257) {
		t.Fatalf("expected default permission to sync from YAML, got %d", merged[0].DefaultPermission)
	}
	if merged[0].Permission != auth.AuthPermission(16258) {
		t.Fatalf("expected explicit table permission to sync from YAML, got %d", merged[0].Permission)
	}
}
