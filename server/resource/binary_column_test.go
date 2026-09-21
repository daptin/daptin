package resource

import (
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/table_info"
)

func TestBinaryColumnValueUsesStableCloudStorageName(t *testing.T) {
	calendarResource := &DbResource{tableInfo: &table_info.TableInfo{
		TableName: "calendar",
		Columns: []api2go.ColumnInfo{{
			Name:         "content",
			ColumnName:   "content",
			ColumnType:   "file.ical",
			DataType:     "longblob",
			IsForeignKey: true,
			ForeignKeyData: api2go.ForeignKeyData{
				DataSource: "cloud_store",
				Namespace:  "dav-storage",
				KeyName:    "calendar",
			},
		}},
	}}
	root := &DbResource{Cruds: map[string]*DbResource{"calendar": calendarResource}}
	resourcePath := "/caldav/11111111-1111-1111-1111-111111111111/calendars/personal/event.ics"

	first := root.binaryColumnValueForStorage("calendar", "content", []byte("first"), resourcePath, "text/calendar").([]interface{})[0].(map[string]interface{})
	second := root.binaryColumnValueForStorage("calendar", "content", []byte("second"), resourcePath, "text/calendar").([]interface{})[0].(map[string]interface{})
	if first["name"] != second["name"] {
		t.Fatalf("same DAV resource generated different storage names: %q != %q", first["name"], second["name"])
	}
	if first["name"] == "event.ics" {
		t.Fatal("storage name did not include the full DAV resource identity")
	}
}
