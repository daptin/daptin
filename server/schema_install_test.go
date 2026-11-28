package server

import "testing"

func TestParseSchemaRejectsMalformedImports(t *testing.T) {
	for _, body := range []string{`{"Imports":[{"Entity":"agprobe","FilePath":"","FileType":"json"}]}`, `null`, `[]`, `{"Tables":[{"TableName":"agprobe","Columns":[{"Name":"name"},{"Name":"name"}]}]}`} {
		if _, err := parseSchemaDocument("schema.json", []byte(body)); err == nil {
			t.Fatalf("invalid schema accepted: %s", body)
		}
	}
	config, err := parseSchemaDocument("schema.yaml", []byte("Tables:\n  - TableName: agprobe\n    Columns:\n      - {Name: name, ColumnType: label, DataType: varchar(100)}\n"))
	if err != nil || config.Tables[0].Columns[0].ColumnName != "name" {
		t.Fatalf("upload normalization: %+v %v", config, err)
	}
}
