package resource

import (
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/table_info"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestValidateUpdateVersion(t *testing.T) {
	tests := []struct {
		name      string
		requested interface{}
		supplied  bool
		status    int
	}{
		{name: "omitted"},
		{name: "current GraphQL integer", requested: int(2), supplied: true},
		{name: "stale GraphQL integer", requested: int(1), supplied: true, status: http.StatusConflict},
		{name: "current integer", requested: int64(2), supplied: true},
		{name: "current JSON number", requested: float64(2), supplied: true},
		{name: "stale", requested: int64(1), supplied: true, status: http.StatusConflict},
		{name: "future", requested: int64(3), supplied: true, status: http.StatusConflict},
		{name: "zero", requested: int64(0), supplied: true, status: http.StatusUnprocessableEntity},
		{name: "negative", requested: int64(-1), supplied: true, status: http.StatusUnprocessableEntity},
		{name: "fraction", requested: 1.5, supplied: true, status: http.StatusUnprocessableEntity},
		{name: "string", requested: "2", supplied: true, status: http.StatusUnprocessableEntity},
		{name: "null", supplied: true, status: http.StatusUnprocessableEntity},
		{name: "overflow", requested: float64(math.MaxInt64), supplied: true, status: http.StatusUnprocessableEntity},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			model := api2go.NewApi2GoModelWithData("widget", nil, 0, nil, map[string]interface{}{"version": int64(2), "name": "old"})
			attrs := map[string]interface{}{"name": "new"}
			if test.supplied {
				attrs["version"] = test.requested
			}
			model.SetAttributes(attrs)
			err := validateUpdateVersion(model)
			if test.status == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var httpErr api2go.HTTPError
			if !errors.As(err, &httpErr) || httpErr.Status() != test.status {
				t.Fatalf("error = %v, want HTTP %d", err, test.status)
			}
		})
	}
}

// A database-layer fixture forces a lost conditional write after the action's
// version precheck, so the SQL guard's conflict must also become HTTP 409.
func TestActionPatchMapsSQLVersionConflict(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`create table widget (
  id integer primary key, reference_id blob not null unique,
  name text, version integer, updated_at timestamp
 )`); err != nil {
		t.Fatal(err)
	}
	ref := daptinid.DaptinReferenceId(uuid.New())
	if _, err := db.Exec(`insert into widget (id, reference_id, name, version, updated_at) values (?, ?, ?, ?, ?)`, 1, ref[:], "old", 1, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`create trigger skip_widget_update before update on widget begin select raise(ignore); end`); err != nil {
		t.Fatal(err)
	}
	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", ColumnType: "label"},
		{Name: "reference_id", ColumnName: "reference_id"},
		{Name: "version", ColumnName: "version"},
		{Name: "updated_at", ColumnName: "updated_at"},
	}
	crud := &DbResource{
		model:      api2go.NewApi2GoModel("widget", columns, 0, nil),
		tableInfo:  &table_info.TableInfo{TableName: "widget", Columns: columns},
		connection: db, ms: &MiddlewareSet{},
	}
	crud.Cruds = map[string]*DbResource{"widget": crud}
	outcome := actionresponse.Outcome{Type: "widget", Method: "PATCH", Attributes: map[string]interface{}{
		"reference_id": ref.String(), "name": "new", "version": int64(1),
	}}
	model, _, err := BuildOutcome(map[string]interface{}{}, outcome, &auth.SessionUser{})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	result, responses, err := crud.executeActionCRUDOutcome(outcome, *model, api2go.Request{PlainRequest: httptest.NewRequest(http.MethodPatch, "/widget", nil)}, tx)
	var httpErr api2go.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status() != http.StatusConflict {
		t.Fatalf("error = %v, want HTTP 409", err)
	}
	if result != nil || len(responses) != 0 {
		t.Fatalf("conflict produced success: %v %v", result, responses)
	}
	var name string
	var version int
	if err := tx.QueryRow(`select name, version from widget where reference_id = ?`, ref[:]).Scan(&name, &version); err != nil {
		t.Fatal(err)
	}
	if name != "old" || version != 1 {
		t.Fatalf("conflict changed row to %s version %d", name, version)
	}
}
