package server

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/columns"
	"github.com/daptin/daptin/server/table_info"
)

func TestLoadConfigFilesTracksExplicitTableFields(t *testing.T) {
	tempDir := t.TempDir()
	schema := []byte(`Tables:
  - TableName: certificate
    IsHidden: false
    Permission: 561408
`)
	if err := os.WriteFile(filepath.Join(tempDir, "schema_test.yaml"), schema, 0600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	t.Setenv("DAPTIN_SCHEMA_FOLDER", tempDir)

	config, errs := LoadConfigFiles()
	if len(errs) > 0 {
		t.Fatalf("load config errors: %v", errs)
	}

	var certificateOverride *table_info.TableInfo
	for i := range config.Tables {
		table := &config.Tables[i]
		if table.TableName == "certificate" && table.Permission == auth.AuthPermission(561408) {
			certificateOverride = table
			break
		}
	}
	if certificateOverride == nil {
		t.Fatalf("expected certificate schema override to be loaded")
	}
	if !certificateOverride.ExplicitFields["IsHidden"] || !certificateOverride.ExplicitFields["is_hidden"] {
		t.Fatalf("expected explicit IsHidden field presence to be tracked, got %#v", certificateOverride.ExplicitFields)
	}
	if certificateOverride.IsHidden {
		t.Fatalf("expected explicit IsHidden=false value to be preserved")
	}
}

func TestLoadConfigFilesSkipsUnsupportedSchemaExtensions(t *testing.T) {
	tempDir := t.TempDir()
	tomlSchema := []byte(`[Tables]
TableName = "certificate"
Permission = 561408
`)
	if err := os.WriteFile(filepath.Join(tempDir, "schema_test.toml"), tomlSchema, 0600); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	t.Setenv("DAPTIN_SCHEMA_FOLDER", tempDir)

	config, errs := LoadConfigFiles()
	if len(errs) > 0 {
		t.Fatalf("load config errors: %v", errs)
	}

	for _, table := range config.Tables {
		if table.TableName == "certificate" && table.Permission == auth.AuthPermission(561408) {
			t.Fatalf("toml schema should not be loaded")
		}
	}
}

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

func TestMergeTablesMergesNewDuplicateConfigTables(t *testing.T) {
	initConfigTables := []table_info.TableInfo{
		{
			TableName:    "built_in",
			IsHidden:     true,
			DefaultOrder: "+created_at",
			Columns: []api2go.ColumnInfo{
				{ColumnName: "hostname", ColumnType: "label", DataType: "varchar(100)", IsIndexed: true},
			},
		},
		{
			TableName:  "built_in",
			IsHidden:   true,
			Permission: auth.AuthPermission(561408),
			ExplicitFields: map[string]bool{
				"IsHidden": true,
			},
		},
	}

	merged := MergeTables(nil, initConfigTables)

	if len(merged) != 1 {
		t.Fatalf("expected duplicate config tables to merge, got %#v", merged)
	}
	if !merged[0].IsHidden {
		t.Fatalf("expected explicit IsHidden override to apply")
	}
	if merged[0].DefaultOrder != "+created_at" {
		t.Fatalf("expected missing DefaultOrder override to preserve standard value, got %q", merged[0].DefaultOrder)
	}
	if merged[0].Permission != auth.AuthPermission(561408) {
		t.Fatalf("expected explicit Permission override to apply, got %d", merged[0].Permission)
	}
	if len(merged[0].Columns) != 1 || merged[0].Columns[0].ColumnName != "hostname" {
		t.Fatalf("expected built-in columns to be preserved, got %#v", merged[0].Columns)
	}
}

func TestMergeTablesExplicitFalseOverrideIsPreserved(t *testing.T) {
	initConfigTables := []table_info.TableInfo{
		{
			TableName: "built_in",
			IsHidden:  true,
		},
		{
			TableName: "built_in",
			IsHidden:  false,
			ExplicitFields: map[string]bool{
				"IsHidden": true,
			},
		},
	}

	merged := MergeTables(nil, initConfigTables)

	if len(merged) != 1 {
		t.Fatalf("expected duplicate config tables to merge, got %#v", merged)
	}
	if merged[0].IsHidden {
		t.Fatalf("expected explicit IsHidden=false override to apply")
	}
}

func TestMergeTablesPermissionOnlyCertificateSchemaPreservesBuiltInColumns(t *testing.T) {
	initConfigTables := []table_info.TableInfo{
		{
			TableName:     "certificate",
			DefaultGroups: table_info.DefaultGroups("administrators"),
			Columns: []api2go.ColumnInfo{
				{ColumnName: "hostname", ColumnType: "label", DataType: "varchar(100)", IsIndexed: true, IsUnique: true},
				{ColumnName: "certificate_pem", ColumnType: "content", DataType: "text", IsNullable: true},
			},
		},
		{
			TableName:  "certificate",
			Permission: auth.AuthPermission(561408),
		},
	}

	merged := MergeTables(nil, initConfigTables)

	if len(merged) != 1 {
		t.Fatalf("expected one certificate table, got %#v", merged)
	}
	if merged[0].Permission != auth.AuthPermission(561408) {
		t.Fatalf("expected explicit certificate permission override, got %d", merged[0].Permission)
	}
	if len(merged[0].Columns) != 2 {
		t.Fatalf("expected certificate columns to be preserved, got %#v", merged[0].Columns)
	}
	hostnameColumn := merged[0].Columns[0]
	if hostnameColumn.ColumnName != "hostname" || hostnameColumn.ColumnType != "label" || hostnameColumn.DataType != "varchar(100)" {
		t.Fatalf("expected hostname column metadata to be preserved, got %#v", hostnameColumn)
	}
	if !hostnameColumn.IsIndexed || !hostnameColumn.IsUnique {
		t.Fatalf("expected hostname indexes to be preserved, got %#v", hostnameColumn)
	}
}

func TestMergeTablesClearsExplicitEmptyCollectionFields(t *testing.T) {
	existingTables := []table_info.TableInfo{
		{
			TableName:     "schema_clear_probe",
			DefaultGroups: table_info.DefaultGroups("administrators"),
			DefaultRelations: map[string][]string{
				"administrators": {"can_edit"},
			},
			Validations: []columns.ColumnTag{
				{ColumnName: "name", Tags: "required"},
			},
			Conformations: []columns.ColumnTag{
				{ColumnName: "name", Tags: "trim"},
			},
			CompositeKeys: [][]string{{"name"}},
		},
	}
	initConfigTables := []table_info.TableInfo{
		{
			TableName:        "schema_clear_probe",
			DefaultGroups:    table_info.DefaultGroupList{},
			DefaultRelations: map[string][]string{},
			Validations:      []columns.ColumnTag{},
			Conformations:    []columns.ColumnTag{},
			CompositeKeys:    [][]string{},
		},
	}

	merged := MergeTables(existingTables, initConfigTables)

	if len(merged[0].DefaultGroups) != 0 {
		t.Fatalf("expected explicit empty DefaultGroups to clear state, got %#v", merged[0].DefaultGroups)
	}
	if len(merged[0].DefaultRelations) != 0 {
		t.Fatalf("expected explicit empty DefaultRelations to clear state, got %#v", merged[0].DefaultRelations)
	}
	if len(merged[0].Validations) != 0 {
		t.Fatalf("expected explicit empty Validations to clear state, got %#v", merged[0].Validations)
	}
	if len(merged[0].Conformations) != 0 {
		t.Fatalf("expected explicit empty Conformations to clear state, got %#v", merged[0].Conformations)
	}
	if len(merged[0].CompositeKeys) != 0 {
		t.Fatalf("expected explicit empty CompositeKeys to clear state, got %#v", merged[0].CompositeKeys)
	}
}

func TestMergeTablesAppliesSchemaOverrideAfterStandardTableForExistingTable(t *testing.T) {
	existingTables := []table_info.TableInfo{
		{
			TableName:    "mail",
			Icon:         "fa-envelope",
			DefaultOrder: "+subject",
			Columns: []api2go.ColumnInfo{
				{
					ColumnName:        "mail",
					ColumnDescription: "Raw message blob",
					ColumnType:        "gzip",
					DataType:          "blob",
					DefaultValue:      "empty-message",
					IsIndexed:         true,
					IsNullable:        true,
					IsUnique:          true,
					Options: []api2go.ValueOptions{
						{ValueType: "string", Value: "raw", Label: "Raw"},
					},
				},
			},
		},
	}
	initConfigTables := []table_info.TableInfo{
		{
			TableName:    "mail",
			Icon:         "fa-envelope",
			DefaultOrder: "+subject",
			Columns: []api2go.ColumnInfo{
				{
					ColumnName:        "mail",
					ColumnDescription: "Raw message blob",
					ColumnType:        "gzip",
					DataType:          "blob",
					DefaultValue:      "empty-message",
					IsIndexed:         true,
					IsNullable:        true,
					IsUnique:          true,
					Options: []api2go.ValueOptions{
						{ValueType: "string", Value: "raw", Label: "Raw"},
					},
				},
			},
		},
		{
			TableName:        "mail",
			TableDescription: "Cloud-backed mail messages",
			Metering: &table_info.MeteringConfig{
				Enabled:   true,
				CostExpr:  "response.bytes",
				MeterType: "mail_storage",
			},
			Columns: []api2go.ColumnInfo{
				{
					ColumnName:   "mail",
					IsForeignKey: true,
					ForeignKeyData: api2go.ForeignKeyData{
						DataSource: "cloud_store",
						Namespace:  "canaster-mail",
						KeyName:    "mail-messages",
					},
				},
			},
		},
	}

	merged := MergeTables(existingTables, initConfigTables)

	if len(merged) != 1 || len(merged[0].Columns) != 1 {
		t.Fatalf("expected one merged mail table/column, got %#v", merged)
	}
	column := merged[0].Columns[0]
	if !column.IsForeignKey {
		t.Fatalf("expected schema override to set existing mail column as a foreign key")
	}
	if column.ForeignKeyData.DataSource != "cloud_store" || column.ForeignKeyData.Namespace != "canaster-mail" || column.ForeignKeyData.KeyName != "mail-messages" {
		t.Fatalf("expected cloud_store override to apply, got %#v", column.ForeignKeyData)
	}
	if column.ColumnType != "gzip" || column.DataType != "blob" {
		t.Fatalf("expected partial column override to preserve type metadata, got column_type=%q data_type=%q", column.ColumnType, column.DataType)
	}
	if column.DefaultValue != "empty-message" {
		t.Fatalf("expected partial column override to preserve default value, got %q", column.DefaultValue)
	}
	if column.ColumnDescription != "Raw message blob" {
		t.Fatalf("expected partial column override to preserve description, got %q", column.ColumnDescription)
	}
	if !column.IsIndexed || !column.IsNullable || !column.IsUnique {
		t.Fatalf("expected partial column override to preserve boolean metadata, got indexed=%t nullable=%t unique=%t", column.IsIndexed, column.IsNullable, column.IsUnique)
	}
	if len(column.Options) != 1 || column.Options[0].Label != "Raw" {
		t.Fatalf("expected partial column override to preserve options, got %#v", column.Options)
	}
	if merged[0].Metering == nil {
		t.Fatalf("expected schema override to preserve metering for existing mail table")
	}
	if merged[0].Metering.CostExpr != "response.bytes" || merged[0].Metering.MeterType != "mail_storage" {
		t.Fatalf("expected metering override to apply to existing table, got %#v", merged[0].Metering)
	}
	if merged[0].TableDescription != "Cloud-backed mail messages" {
		t.Fatalf("expected table description override to apply to existing table, got %q", merged[0].TableDescription)
	}
	if merged[0].Icon != "fa-envelope" {
		t.Fatalf("expected partial override to preserve standard mail icon, got %q", merged[0].Icon)
	}
	if merged[0].DefaultOrder != "+subject" {
		t.Fatalf("expected partial override to preserve default order, got %q", merged[0].DefaultOrder)
	}
}
