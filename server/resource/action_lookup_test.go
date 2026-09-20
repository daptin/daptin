package resource

import (
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestActionLookupDistinguishesMissingActionFromLookupFailure(t *testing.T) {
	db, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		`create table world (id integer primary key, table_name text)`,
		`create table action (action_name text, label text, world_id integer, action_schema text, instance_optional bool, reference_id blob)`,
		`insert into world (id, table_name) values (1, 'cloud_store')`,
		`insert into action (action_name, label, world_id, action_schema, instance_optional, reference_id)
		 values ('upload_file', 'Upload file', 1, '{}', true, randomblob(16))`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}

	crud := &DbResource{}
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	action, err := crud.GetActionByName("cloud_store", "upload_file", tx)
	if err != nil || action.Name != "upload_file" {
		t.Fatalf("known action = %#v, %v", action, err)
	}
	_, err = crud.GetActionByName("cloud_store", "cloudstore_file_upload", tx)
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("missing action error = %v, want sql.ErrNoRows", err)
	}

	request := api2go.Request{PlainRequest: httptest.NewRequest(http.MethodPost, "/action/cloud_store/cloudstore_file_upload", nil)}
	responses, err := crud.HandleActionRequest(actionresponse.ActionRequest{
		Type: "cloud_store", Action: "cloudstore_file_upload",
	}, request, tx)
	assertActionLookupHTTPError(t, responses, err, http.StatusNotFound)

	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	responses, err = crud.HandleActionRequest(actionresponse.ActionRequest{
		Type: "cloud_store", Action: "upload_file",
	}, request, tx)
	assertActionLookupHTTPError(t, responses, err, http.StatusInternalServerError)
}

func assertActionLookupHTTPError(t *testing.T, responses []actionresponse.ActionResponse, err error, wantStatus int) {
	t.Helper()
	if len(responses) != 0 {
		t.Fatalf("lookup failure produced responses: %#v", responses)
	}
	var httpErr api2go.HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status() != wantStatus {
		t.Fatalf("lookup failure = %v, want HTTP %d", err, wantStatus)
	}
}
