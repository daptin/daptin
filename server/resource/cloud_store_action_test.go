package resource

import "testing"

func TestCloudStoreActionsPassPersistedStoreTypeToPerformers(t *testing.T) {
	want := map[string]bool{
		"upload_file":   true,
		"create_site":   true,
		"delete_path":   true,
		"create_folder": true,
		"move_path":     true,
	}

	for _, action := range SystemActions {
		if !want[action.Name] {
			continue
		}
		delete(want, action.Name)
		if len(action.OutFields) != 1 || action.OutFields[0].Attributes["store_type"] != "$.store_type" {
			t.Errorf("action %q does not pass the persisted store type to its performer", action.Name)
		}
	}
	for name := range want {
		t.Errorf("cloud store action %q is missing", name)
	}
}

func TestSiteStorageSyncUsesOnlyServerManagedCache(t *testing.T) {
	for _, action := range SystemActions {
		if action.Name != "sync_site_storage" {
			continue
		}
		if len(action.InFields) != 0 {
			t.Fatal("sync_site_storage accepts caller-controlled input")
		}
		if len(action.OutFields) != 1 {
			t.Fatalf("sync_site_storage outcomes = %d, want 1", len(action.OutFields))
		}
		if _, ok := action.OutFields[0].Attributes["path"]; ok {
			t.Fatal("sync_site_storage passes a caller-controlled path to its performer")
		}
		return
	}
	t.Fatal("sync_site_storage action is missing")
}
