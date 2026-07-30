package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	"github.com/daptin/daptin/server/table_info"
)

func TestCheckAuditTablesGeneratesSafeSchema(t *testing.T) {
	config := CmsConfig{Tables: []table_info.TableInfo{{
		TableName:      "account",
		IsAuditEnabled: true,
		Columns: []api2go.ColumnInfo{
			{Name: "id", ColumnName: "id", ColumnType: "id", DataType: "INTEGER"},
			{Name: "name", ColumnName: "name", ColumnType: "label", DataType: "varchar(80)", IsUnique: true},
			{Name: "password", ColumnName: "password", ColumnType: "password", DataType: "varchar(100)"},
			{Name: "token", ColumnName: "token", ColumnType: "encrypted", DataType: "varchar(500)"},
			{Name: "attachment", ColumnName: "attachment", ColumnType: "file.*", DataType: "longblob"},
			{Name: "payload", ColumnName: "payload", ColumnType: "content", DataType: "blob"},
			{Name: "permission", ColumnName: "permission", ColumnType: "measurement", DataType: "bigint"},
			{Name: "owner", ColumnName: USER_ACCOUNT_ID_COLUMN, ColumnType: "measurement", DataType: "bigint"},
			{Name: "project_id", ColumnName: "project_id", IsForeignKey: true, DataType: "bigint", ForeignKeyData: api2go.ForeignKeyData{Namespace: "project", KeyName: "id"}},
		},
	}}}

	CheckAuditTables(&config)

	if len(config.Tables) != 2 {
		t.Fatalf("expected generated audit table, got %d tables", len(config.Tables))
	}
	audit := config.Tables[1]
	if audit.Permission != AuditTablePermission || audit.Permission&(auth.GuestCreate|auth.GuestRead|auth.GuestPeek) != 0 {
		t.Fatalf("unsafe audit table permission: %d", audit.Permission)
	}
	if audit.DefaultPermission != AuditRowDefaultPermission || audit.DefaultPermission&(auth.GuestCreate|auth.GuestRead|auth.GuestPeek) != 0 {
		t.Fatalf("unsafe audit row default permission: %d", audit.DefaultPermission)
	}

	columns := make(map[string]api2go.ColumnInfo)
	for _, column := range audit.Columns {
		if _, exists := columns[column.ColumnName]; exists {
			t.Fatalf("duplicate generated audit column %q", column.ColumnName)
		}
		columns[column.ColumnName] = column
	}
	for _, excluded := range []string{"id", "password", "token", "attachment", "payload", "permission", USER_ACCOUNT_ID_COLUMN} {
		if _, exists := columns[excluded]; exists {
			t.Errorf("sensitive column %q was copied to audit schema", excluded)
		}
	}
	for _, required := range []string{"name", "project_id", "source_reference_id", "operation"} {
		if _, exists := columns[required]; !exists {
			t.Errorf("required column %q missing from audit schema", required)
		}
	}
	project := columns["project_id"]
	if project.IsForeignKey || project.ForeignKeyData != (api2go.ForeignKeyData{}) || project.DataType != "varchar" {
		t.Fatalf("audit foreign key was not normalized: %#v", project)
	}
	if columns["name"].IsUnique {
		t.Fatal("audit copy retained source uniqueness")
	}
}

func TestCheckAuditTablesPreservesExplicitAuditTable(t *testing.T) {
	explicitPermission := auth.GuestPeek | auth.GuestRead
	config := CmsConfig{Tables: []table_info.TableInfo{
		{
			TableName:      "account",
			IsAuditEnabled: true,
			Columns: []api2go.ColumnInfo{
				{Name: "name", ColumnName: "name"},
				{Name: "email", ColumnName: "email"},
			},
		},
		{
			TableName:         "account_audit",
			Permission:        explicitPermission,
			DefaultPermission: explicitPermission,
			Columns: []api2go.ColumnInfo{
				{Name: "name", ColumnName: "name"},
			},
		},
	}}

	CheckAuditTables(&config)

	if len(config.Tables) != 2 {
		t.Fatalf("explicit audit table was replaced: %d tables", len(config.Tables))
	}
	audit := config.Tables[1]
	if len(audit.Columns) != 1 || audit.Columns[0].ColumnName != "name" {
		t.Fatalf("explicit audit columns were changed: %#v", audit.Columns)
	}
	if audit.Permission != explicitPermission || audit.DefaultPermission != explicitPermission {
		t.Fatalf("explicit audit permissions were changed: %#v", audit)
	}
}
