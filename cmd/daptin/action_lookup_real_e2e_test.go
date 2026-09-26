package main

import (
	"net/http"
	"testing"
)

func TestUnknownActionRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	baseURL, client := fixture.URL, fixture.Client
	token := fixture.SignupAdmin(t)
	storeID := accessGroupsE2ECreateRecord(t, client, baseURL, token, "cloud_store", map[string]interface{}{
		"name": "action-lookup-store", "store_type": "local", "store_provider": "local",
		"root_path": t.TempDir(), "store_parameters": "{}",
	})
	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/cloud_store/cloudstore_file_upload", token,
		map[string]interface{}{"attributes": map[string]interface{}{"cloud_store_id": storeID}}, http.StatusNotFound)
	if items, ok := response.([]interface{}); ok && len(items) == 0 {
		t.Fatal("unknown action returned an empty success response")
	}
}
