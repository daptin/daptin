package main

import (
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

const exportCSVRealE2ESchema = `
Tables:
  - TableName: export_csv_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
`

func TestExportCSVActionsRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: exportCSVRealE2ESchema})
	token := fixture.SignupAdmin(t)
	empty := transportE2EPostJSON(t, fixture.Client,
		fixture.URL+"/action/world/export_csv_data", token,
		map[string]interface{}{"attributes": map[string]interface{}{"table_name": "export_csv_probe"}})
	emptyContent, ok := transportE2EFindString(empty, "content")
	if !ok || emptyContent != "" {
		t.Fatalf("empty table should return an empty CSV download: %#v", empty)
	}

	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost,
		fixture.URL+"/api/export_csv_probe", token,
		accessGroupsE2ERecordPayload("export_csv_probe", "", map[string]interface{}{"name": "saved row"}), http.StatusCreated)
	listed := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		fixture.URL+"/api/export_csv_probe", token, nil, http.StatusOK)
	listData := listed.(map[string]interface{})["data"].([]interface{})
	if len(listData) != 1 {
		t.Fatalf("authorized list returned %d records, want 1", len(listData))
	}

	for _, action := range []string{"export_csv_data", "export_data"} {
		t.Run(action, func(t *testing.T) {
			attributes := map[string]interface{}{"table_name": "export_csv_probe"}
			if action == "export_data" {
				attributes["format"] = "csv"
			}
			response := transportE2EPostJSON(t, fixture.Client,
				fixture.URL+"/action/world/"+action, token,
				map[string]interface{}{"attributes": attributes})
			items, ok := response.([]interface{})
			if !ok || len(items) != 1 {
				t.Fatalf("unexpected action response: %#v", response)
			}
			effect := items[0].(map[string]interface{})
			if effect["ResponseType"] != "client.file.download" {
				t.Fatalf("unexpected action effect: %#v", effect)
			}
			wantName, wantType := "daptin_export_export_csv_probe.csv", "text/csv"
			if name, _ := transportE2EFindString(effect, "name"); name != wantName {
				t.Fatalf("download name = %q, want %q", name, wantName)
			}
			if contentType, _ := transportE2EFindString(effect, "contentType"); contentType != wantType {
				t.Fatalf("download content type = %q, want %q", contentType, wantType)
			}
			content, ok := transportE2EFindString(effect, "content")
			if !ok {
				t.Fatalf("download content absent: %#v", effect)
			}
			decoded, err := base64.StdEncoding.DecodeString(content)
			if err != nil {
				t.Fatal(err)
			}
			records, err := csv.NewReader(strings.NewReader(string(decoded))).ReadAll()
			if err != nil {
				t.Fatalf("invalid CSV %q: %v", decoded, err)
			}
			if len(records) != 2 {
				t.Fatalf("CSV has %d records, want headings and saved row: %q", len(records), decoded)
			}
			nameColumn := -1
			for i, heading := range records[0] {
				if heading == "name" {
					nameColumn = i
					break
				}
			}
			if nameColumn < 0 || records[1][nameColumn] != "saved row" {
				t.Fatalf("CSV omits saved row: %q", decoded)
			}
		})
	}

	t.Run("export_data_without_headers", func(t *testing.T) {
		response := transportE2EPostJSON(t, fixture.Client,
			fixture.URL+"/action/world/export_data", token,
			map[string]interface{}{"attributes": map[string]interface{}{
				"table_name": "export_csv_probe", "format": "csv", "include_headers": false,
			}})
		content, ok := transportE2EFindString(response, "content")
		if !ok {
			t.Fatalf("download content absent: %#v", response)
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			t.Fatal(err)
		}
		records, err := csv.NewReader(strings.NewReader(string(decoded))).ReadAll()
		if err != nil || len(records) != 1 {
			t.Fatalf("CSV without headers = %q, records = %#v, error = %v", decoded, records, err)
		}
	})

	t.Run("export_data_default_format", func(t *testing.T) {
		response := transportE2EPostJSON(t, fixture.Client,
			fixture.URL+"/action/world/export_data", token,
			map[string]interface{}{"attributes": map[string]interface{}{"table_name": "export_csv_probe"}})
		contentType, _ := transportE2EFindString(response, "contentType")
		if contentType != "application/json" {
			t.Fatalf("default content type = %q, want application/json", contentType)
		}
		content, ok := transportE2EFindString(response, "content")
		if !ok {
			t.Fatalf("download content absent: %#v", response)
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			t.Fatal(err)
		}
		var exported map[string][]map[string]interface{}
		if err := json.Unmarshal(decoded, &exported); err != nil {
			t.Fatalf("invalid JSON export %q: %v", decoded, err)
		}
		if rows := exported["export_csv_probe"]; len(rows) != 1 || rows[0]["name"] != "saved row" {
			t.Fatalf("default JSON export omits saved row: %#v", exported)
		}
	})

	t.Run("export_data_all_tables", func(t *testing.T) {
		response := transportE2EPostJSON(t, fixture.Client,
			fixture.URL+"/action/world/export_data", token,
			map[string]interface{}{"attributes": map[string]interface{}{}})
		content, ok := transportE2EFindString(response, "content")
		if !ok {
			t.Fatalf("download content absent: %#v", response)
		}
		decoded, err := base64.StdEncoding.DecodeString(content)
		if err != nil {
			t.Fatal(err)
		}
		var exported map[string][]map[string]interface{}
		if err := json.Unmarshal(decoded, &exported); err != nil {
			t.Fatalf("invalid all-table JSON export: %v", err)
		}
		if rows := exported["export_csv_probe"]; len(rows) != 1 || rows[0]["name"] != "saved row" {
			t.Fatalf("all-table export omits saved row: %#v", rows)
		}
	})

	for _, action := range []string{"export_csv_data", "export_data"} {
		t.Run(action+"_missing_table", func(t *testing.T) {
			attributes := map[string]interface{}{"table_name": "missing_export_table"}
			if action == "export_data" {
				attributes["format"] = "csv"
			}
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost,
				fixture.URL+"/action/world/"+action, token,
				map[string]interface{}{"attributes": attributes}, http.StatusInternalServerError)
		})
		t.Run(action+"_invalid_table_name", func(t *testing.T) {
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost,
				fixture.URL+"/action/world/"+action, token,
				map[string]interface{}{"attributes": map[string]interface{}{
					"table_name": map[string]interface{}{"invalid": true},
				}}, http.StatusInternalServerError)
		})
	}
}
