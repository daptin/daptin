package main

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSchemaUploadPreservesLaterActionEditsRealE2E(t *testing.T) {
	requireRealE2E(t)
	f := startDaptinE2E(t, transportE2EDaptinOptions{schema: "Tables:\n  - TableName: agprobe\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"})
	admin := f.SignupAdmin(t)
	action := map[string]interface{}{"Name": "sayhello", "OnType": "agprobe", "InstanceOptional": true, "OutFields": []interface{}{map[string]interface{}{"Type": "review.response", "Method": "ACTIONRESPONSE", "Attributes": map[string]interface{}{"message": "original"}}}}
	schema := mustAgentJSON(map[string]interface{}{"Actions": []interface{}{action}})
	transportE2EPostJSON(t, f.Client, f.URL+"/action/world/upload_system_schema", admin, map[string]interface{}{"attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "action.json", "file": base64.StdEncoding.EncodeToString(schema)}}}})
	restart := func() {
		transportE2EPostJSON(t, f.Client, f.URL+"/action/world/restart_daptin", admin, map[string]interface{}{"attributes": map[string]interface{}{}})
		waitForAdminRestart(t, f.Client, f.URL)
	}
	restart()
	ref := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "action", "action_name", "sayhello")
	before := transportE2EGetJSON(t, f.Client, f.URL+"/api/action/"+ref, admin)
	version, _ := transportE2EPath(before, "data.attributes.version")
	action["OutFields"] = []interface{}{map[string]interface{}{"Type": "review.response", "Method": "ACTIONRESPONSE", "Attributes": map[string]interface{}{"message": "edited"}}}
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPatch, f.URL+"/api/action/"+ref, admin, accessGroupsE2ERecordPayload("action", ref, map[string]interface{}{"version": version, "action_schema": string(mustAgentJSON(action))}), 200)
	exported := transportE2EPostJSON(t, f.Client, f.URL+"/action/action/get_action_schema", admin, map[string]interface{}{"attributes": map[string]interface{}{"action_id": ref}})
	encoded, ok := transportE2EPath(exported, "0.Attributes.content")
	if !ok {
		t.Fatalf("schema export: %s", mustAgentJSON(exported))
	}
	edited, err := base64.StdEncoding.DecodeString(encoded.(string))
	if err != nil || !strings.Contains(string(edited), "edited") {
		t.Fatalf("edit did not persist: %s %v", edited, err)
	}
	restart()
	response := transportE2EPostJSON(t, f.Client, f.URL+"/action/agprobe/sayhello", admin, map[string]interface{}{"attributes": map[string]interface{}{}})
	message, _ := transportE2EPath(response, "0.Attributes.message")
	t.Logf("saved action edit before restart; message after restart=%v", message)
	if message != "edited" {
		t.Fatalf("saved action edit was lost: %v", message)
	}
}

func TestSchemaUploadImportsOnceRealE2E(t *testing.T) {
	requireRealE2E(t)
	f := startDaptinE2E(t, transportE2EDaptinOptions{schema: "Tables:\n  - TableName: agprobe\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"})
	admin := f.SignupAdmin(t)
	path := filepath.Join(t.TempDir(), "seed.csv")
	if err := os.WriteFile(path, []byte("name\nseed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	schema := mustAgentJSON(map[string]interface{}{"Imports": []interface{}{map[string]interface{}{"Entity": "agprobe", "FilePath": path, "FileType": "csv"}}})
	transportE2EPostJSON(t, f.Client, f.URL+"/action/world/upload_system_schema", admin, map[string]interface{}{"attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "import.json", "file": base64.StdEncoding.EncodeToString(schema)}}}})
	counts := []int{}
	for i := 0; i < 2; i++ {
		transportE2EPostJSON(t, f.Client, f.URL+"/action/world/restart_daptin", admin, map[string]interface{}{"attributes": map[string]interface{}{}})
		waitForAdminRestart(t, f.Client, f.URL)
		response := transportE2EGetJSON(t, f.Client, f.URL+"/api/agprobe", admin)
		counts = append(counts, len(accessGroupsE2EDataArray(t, response)))
	}
	t.Logf("imported record counts after two restarts=%v", counts)
	if counts[0] != 1 || counts[1] != 1 {
		t.Fatalf("upload import replayed: %v", counts)
	}
}

func TestSchemaUploadKeyedImportRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []string{"sqlite", "postgres", "mysql"} {
		t.Run(backend, func(t *testing.T) {
			options := reliabilityDatabase(t, backend)
			options.schema = "Tables:\n  - TableName: agseed\n    Columns:\n      - {Name: code, ColumnType: label, DataType: varchar(100), IsUnique: true}\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"
			f := startDaptinE2E(t, options)
			admin := f.SignupAdmin(t)
			for _, format := range []string{"csv", "json"} {
				t.Run(format, func(t *testing.T) {
					path := filepath.Join(t.TempDir(), "seed."+format)
					var initialID string
					for _, value := range []string{"first", "updated", "byref"} {
						body := "code,name\n" + format + "," + value + "\n"
						if format == "json" {
							body = `{"agseed":[{"code":"` + format + `","name":"` + value + `"}]}`
						}
						if value == "byref" {
							body = "reference_id,name\n" + initialID + ",byref\n"
							if format == "json" {
								body = `{"agseed":[{"reference_id":"` + initialID + `","name":"byref"}]}`
							}
						}
						if err := os.WriteFile(path, []byte(body), 0600); err != nil {
							t.Fatal(err)
						}
						schema := mustAgentJSON(map[string]interface{}{"Imports": []interface{}{map[string]interface{}{"Entity": "agseed", "FilePath": path, "FileType": format}}})
						transportE2EPostJSON(t, f.Client, f.URL+"/action/world/upload_system_schema", admin, map[string]interface{}{"attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "import.json", "file": base64.StdEncoding.EncodeToString(schema)}}}})
						if value == "first" {
							initialID = accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "agseed", "code", format)
						}
					}
					response := transportE2EGetJSON(t, f.Client, f.URL+"/api/agseed", admin)
					matches := 0
					for _, row := range accessGroupsE2EDataArray(t, response) {
						code, _ := transportE2EPath(row, "attributes.code")
						if code == format {
							matches++
							name, _ := transportE2EPath(row, "attributes.name")
							id, _ := transportE2EPath(row, "id")
							if name != "byref" || id != initialID {
								t.Fatalf("keyed import did not update row: %s", mustAgentJSON(row))
							}
						}
					}
					if matches != 1 {
						t.Fatalf("matching rows=%d: %s", matches, mustAgentJSON(response))
					}
				})
			}
		})
	}
}

func TestSchemaUploadRejectsBadImportRealE2E(t *testing.T) {
	requireRealE2E(t)
	f := startDaptinE2E(t, transportE2EDaptinOptions{})
	admin := f.SignupAdmin(t)
	body := `{"Tables":[{"TableName":"agbad","Columns":[{"Name":"name","ColumnType":"label","DataType":"varchar(100)"}]}],"Imports":[{"Entity":"agbad","FilePath":"","FileType":"json"}]}`
	status, response := postLLMActionForStatus(t, f.Client, f.URL+"/action/world/upload_system_schema", admin, map[string]interface{}{"attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "bad.json", "file": base64.StdEncoding.EncodeToString([]byte(body))}}}})
	if status < 400 {
		t.Fatalf("malformed import accepted: %d %s", status, response)
	}
	if !strings.Contains(string(response), "import file path is required") {
		t.Fatalf("missing validation error: %d %s", status, response)
	}
	ready, err := f.Client.Get(f.URL + "/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer ready.Body.Close()
	if ready.StatusCode != 200 {
		t.Fatalf("malformed upload made server unavailable: %d", ready.StatusCode)
	}
}

func TestSchemaUploadMergesExistingDefinitionsRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, backend := range []string{"sqlite", "postgres", "mysql"} {
		t.Run(backend, func(t *testing.T) {
			options := reliabilityDatabase(t, backend)
			options.schema = "Tables:\n  - TableName: agmerge\n    IsHidden: true\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"
			f := startDaptinE2E(t, options)
			admin := f.SignupAdmin(t)
			id := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "world", "table_name", "agmerge")
			schema := `{"Tables":[{"TableName":"agmerge","Columns":[{"Name":"extra","ColumnType":"label","DataType":"varchar(100)","IsNullable":true}]}]}`
			transportE2EPostJSON(t, f.Client, f.URL+"/action/world/upload_system_schema", admin, map[string]interface{}{"attributes": map[string]interface{}{"schema_file": []interface{}{map[string]interface{}{"name": "merge.json", "file": base64.StdEncoding.EncodeToString([]byte(schema))}}}})
			row := transportE2EGetJSON(t, f.Client, f.URL+"/api/world/"+id, admin)
			hidden, _ := transportE2EPath(row, "data.attributes.is_hidden")
			if hidden != true && hidden != float64(1) {
				t.Fatalf("upload reset an existing table flag: %s", mustAgentJSON(row))
			}
			transportE2EPostJSON(t, f.Client, f.URL+"/action/world/restart_daptin", admin, map[string]interface{}{"attributes": map[string]interface{}{}})
			waitForAdminRestart(t, f.Client, f.URL)
			ref := accessGroupsE2ECreateRecord(t, f.Client, f.URL, admin, "agmerge", map[string]interface{}{"name": "kept", "extra": "added"})
			row = transportE2EGetJSON(t, f.Client, f.URL+"/api/agmerge/"+ref, admin)
			name, _ := transportE2EPath(row, "data.attributes.name")
			extra, _ := transportE2EPath(row, "data.attributes.extra")
			if name != "kept" || extra != "added" {
				t.Fatalf("upload did not merge columns: %s", mustAgentJSON(row))
			}
		})
	}
}
