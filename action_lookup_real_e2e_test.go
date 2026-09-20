package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnknownActionRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1")
	}
	usedPorts := map[int]bool{}
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	olricPort := freeTransportE2EPortPair(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "action-lookup.db"), olricPort: olricPort,
	})
	defer process.stopProcess()

	client := &http.Client{Timeout: 20 * time.Second}
	token := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
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
