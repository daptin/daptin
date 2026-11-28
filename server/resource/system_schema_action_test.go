package resource

import "testing"

func TestUploadSystemSchemaUsesTransactionalPerformer(t *testing.T) {
	for _, action := range SystemActions {
		if action.Name != "upload_system_schema" {
			continue
		}
		if len(action.OutFields) != 1 || action.OutFields[0].Method != "EXECUTE" {
			t.Fatal("upload_system_schema must execute its transactional performer")
		}
		return
	}
	t.Fatal("upload_system_schema action is missing")
}
