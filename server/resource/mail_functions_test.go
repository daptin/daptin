package resource

import (
	"encoding/base64"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
)

func TestMailColumnBytesAcceptsDBBackedStringAfterCloudStoreConfiguration(t *testing.T) {
	message := []byte("Subject: migrated\r\n\r\nbody")
	encoded := base64.StdEncoding.EncodeToString(message)
	mailResource := &DbResource{
		tableInfo: cloudStoreMailTableInfo("mail"),
	}
	root := &DbResource{
		Cruds: map[string]*DbResource{"mail": mailResource},
	}

	got, err := root.MailColumnBytes("mail", "mail", encoded)
	if err != nil {
		t.Fatalf("MailColumnBytes returned error: %v", err)
	}
	if string(got) != string(message) {
		t.Fatalf("MailColumnBytes = %q, want %q", string(got), string(message))
	}
}

func TestMailColumnBytesCloudStoreRequiresHydratedContents(t *testing.T) {
	message := []byte("Subject: cloud\r\n\r\nbody")
	encoded := base64.StdEncoding.EncodeToString(message)
	mailResource := &DbResource{
		tableInfo: cloudStoreMailTableInfo("outbox"),
	}
	root := &DbResource{
		Cruds: map[string]*DbResource{"outbox": mailResource},
	}

	storedMetadata := []map[string]interface{}{{
		"name": "queued-message.eml",
		"path": "mail-storage/mail-messages/queued-message.eml",
		"type": mailMessageFileType,
	}}
	if _, err := root.MailColumnBytes("outbox", "mail", storedMetadata); err == nil {
		t.Fatalf("expected stored cloud-store metadata without contents to fail")
	}

	hydratedMetadata := []map[string]interface{}{{
		"name":     "queued-message.eml",
		"path":     "mail-storage/mail-messages/queued-message.eml",
		"type":     mailMessageFileType,
		"contents": encoded,
	}}
	got, err := root.MailColumnBytes("outbox", "mail", hydratedMetadata)
	if err != nil {
		t.Fatalf("MailColumnBytes returned error for hydrated cloud-store mail: %v", err)
	}
	if string(got) != string(message) {
		t.Fatalf("MailColumnBytes = %q, want %q", string(got), string(message))
	}
}

func TestResultToArrayCloudStoreMailColumnAcceptsDBBackedString(t *testing.T) {
	message := []byte("Subject: migrated\r\n\r\nbody")
	encoded := base64.StdEncoding.EncodeToString(message)
	dbResource := &DbResource{
		tableInfo: cloudStoreMailTableInfo("mail"),
	}

	rows, includes, err := dbResource.ResultToArrayOfMapWithTransaction(
		[]map[string]interface{}{{
			"__type":  "mail",
			"mail":    encoded,
			"mail_id": "migrated-message",
		}},
		map[string]api2go.ColumnInfo{
			"mail": cloudStoreMailColumn(),
		},
		map[string]bool{"mail": true},
		nil,
	)
	if err != nil {
		t.Fatalf("ResultToArrayOfMapWithTransaction returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	files, ok := rows[0]["mail"].([]map[string]interface{})
	if !ok || len(files) != 1 {
		t.Fatalf("expected mail file list, got %#v", rows[0]["mail"])
	}
	if files[0]["contents"] != encoded {
		t.Fatalf("expected inline contents to be preserved, got %#v", files[0])
	}
	if files[0]["name"] != "migrated-message.eml" {
		t.Fatalf("expected stable file name, got %#v", files[0]["name"])
	}
	if len(includes) != 1 || len(includes[0]) != 1 {
		t.Fatalf("expected one local include, got %#v", includes)
	}
	if includes[0][0]["__type"] != "gzip" {
		t.Fatalf("expected include type gzip, got %#v", includes[0][0]["__type"])
	}
}

func cloudStoreMailTableInfo(tableName string) *table_info.TableInfo {
	return &table_info.TableInfo{
		TableName: tableName,
		Columns: []api2go.ColumnInfo{
			cloudStoreMailColumn(),
		},
	}
}

func cloudStoreMailColumn() api2go.ColumnInfo {
	return api2go.ColumnInfo{
		Name:         "mail",
		ColumnName:   "mail",
		ColumnType:   "gzip",
		DataType:     "blob",
		IsForeignKey: true,
		ForeignKeyData: api2go.ForeignKeyData{
			DataSource: "cloud_store",
			Namespace:  "mail-storage",
			KeyName:    "mail-messages",
		},
	}
}
