package resource

import "testing"

func TestUploadSystemSchemaReturnsItsBuiltNotification(t *testing.T) {
	for _, action := range SystemActions {
		if action.Name != "upload_system_schema" {
			continue
		}
		if len(action.OutFields) != 1 || action.OutFields[0].Method != "ACTIONRESPONSE" {
			t.Fatal("upload_system_schema must return the notification built by system_json_schema_update")
		}
		return
	}
	t.Fatal("upload_system_schema action is missing")
}
