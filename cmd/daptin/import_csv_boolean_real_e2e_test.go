package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/artpar/xlsx/v2"
)

const importBooleanRealE2ESchema = `
Tables:
  - TableName: ibprobe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label
      - Name: completed
        DataType: bool
        ColumnType: truefalse
      - Name: note
        DataType: varchar(100)
        ColumnType: label
`

func TestImportCSVBooleanRealE2E(t *testing.T) {
	backends := []struct {
		name, databaseType, connectionString string
	}{
		{name: "sqlite"},
		{name: "postgres", databaseType: "postgres", connectionString: os.Getenv("DAPTIN_IMPORT_POSTGRES_DSN")},
		{name: "mariadb", databaseType: "mysql", connectionString: os.Getenv("DAPTIN_IMPORT_MYSQL_DSN")},
	}

	for _, backend := range backends {
		t.Run(backend.name, func(t *testing.T) {
			if backend.databaseType != "" && backend.connectionString == "" {
				t.Skip("database connection string is not configured")
			}
			fixture := startDaptinE2E(t, transportE2EDaptinOptions{
				databaseType: backend.databaseType, connectionString: backend.connectionString, schema: importBooleanRealE2ESchema,
			})
			token := fixture.SignupAdmin(t)
			worldID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, token,
				"world", "table_name", "ibprobe")

			importBooleanFile(t, fixture, token, worldID, "boolean.csv", "text/csv",
				[]byte("title,completed,note\nCSV false,false,true\nCSV true,true,false\nCSV zero,0,true\nCSV one,1,false\n"), 4)
			importBooleanFile(t, fixture, token, worldID, "boolean.json", "application/json",
				[]byte(`[{"title":"JSON false","completed":false,"note":"true"},{"title":"JSON true","completed":true,"note":"false"}]`), 2)

			book := xlsx.NewFile()
			sheet, err := book.AddSheet("ibprobe")
			if err != nil {
				t.Fatal(err)
			}
			for _, values := range [][]string{
				{"title", "completed", "note"},
				{"XLSX false", "false", "true"},
				{"XLSX true", "true", "false"},
			} {
				row := sheet.AddRow()
				for _, value := range values {
					row.AddCell().SetString(value)
				}
			}
			var xlsxData bytes.Buffer
			if err := book.Write(&xlsxData); err != nil {
				t.Fatal(err)
			}
			importBooleanFile(t, fixture, token, worldID, "boolean.xlsx",
				"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", xlsxData.Bytes(), 2)

			response := transportE2EGetJSON(t, fixture.Client,
				fixture.URL+"/api/ibprobe?page%5Bsize%5D=100", token)
			want := map[string]bool{
				"CSV false": false, "CSV true": true, "CSV zero": false, "CSV one": true,
				"JSON false": false, "JSON true": true, "XLSX false": false, "XLSX true": true,
			}
			rows := accessGroupsE2EDataArray(t, response)
			if len(rows) != len(want) {
				t.Fatalf("got %d imported rows, want %d: %#v", len(rows), len(want), response)
			}
			for _, item := range rows {
				attributes := item.(map[string]interface{})["attributes"].(map[string]interface{})
				title := attributes["title"].(string)
				completed, found := want[title]
				if !found {
					t.Fatalf("unexpected imported row %q", title)
				}
				if got := fmt.Sprint(attributes["completed"]); got != fmt.Sprint(completed) && got != fmt.Sprint(map[bool]int{false: 0, true: 1}[completed]) {
					t.Errorf("%s: completed = %v, want %v", title, attributes["completed"], completed)
				}
				if got := attributes["note"]; got != map[bool]string{false: "true", true: "false"}[completed] {
					t.Errorf("%s: note = %v", title, got)
				}
			}
		})
	}
}

func importBooleanFile(t *testing.T, fixture *daptinE2EFixture, token, worldID, name, contentType string, data []byte, wantRows int) {
	t.Helper()
	response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost,
		fixture.URL+"/action/world/import_data", token, map[string]interface{}{
			"attributes": map[string]interface{}{
				"world_id": worldID,
				"dump_file": []interface{}{map[string]interface{}{
					"name": name, "file": "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(data),
				}},
			},
		}, http.StatusOK)
	items, ok := response.([]interface{})
	if !ok || len(items) != 1 {
		t.Fatalf("unexpected import response: %#v", response)
	}
	attributes := items[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	if got := attributes["rows_imported"]; got != float64(wantRows) {
		t.Fatalf("%s: imported %v rows, want %d: %#v", name, got, wantRows, response)
	}
}
