package server

import (
	"testing"

	"github.com/artpar/api2go/v2"
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

func TestMergeTablesPreservesExistingCloudStoreColumnConfiguration(t *testing.T) {
	existingTables := []table_info.TableInfo{
		{
			TableName: "mail",
			Columns: []api2go.ColumnInfo{
				{
					ColumnName:   "mail",
					ColumnType:   "gzip",
					DataType:     "blob",
					IsForeignKey: true,
					ForeignKeyData: api2go.ForeignKeyData{
						DataSource: "cloud_store",
						Namespace:  "localstore",
						KeyName:    "mail-messages",
					},
				},
			},
		},
	}
	initConfigTables := []table_info.TableInfo{
		{
			TableName: "mail",
			Columns: []api2go.ColumnInfo{
				{
					ColumnName: "mail",
					ColumnType: "gzip",
					DataType:   "blob",
				},
			},
		},
	}

	merged := MergeTables(existingTables, initConfigTables)

	if len(merged) != 1 || len(merged[0].Columns) != 1 {
		t.Fatalf("expected one merged table/column, got %#v", merged)
	}
	column := merged[0].Columns[0]
	if !column.IsForeignKey {
		t.Fatalf("expected existing cloud_store column to remain a foreign key")
	}
	if column.ForeignKeyData.DataSource != "cloud_store" {
		t.Fatalf("expected cloud_store data source to be preserved, got %q", column.ForeignKeyData.DataSource)
	}
	if column.ForeignKeyData.Namespace != "localstore" || column.ForeignKeyData.KeyName != "mail-messages" {
		t.Fatalf("expected cloud_store target to be preserved, got %#v", column.ForeignKeyData)
	}
}
