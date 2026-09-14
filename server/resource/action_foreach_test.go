package resource

import (
	"bytes"
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

func TestNormalizeActionForeachItems(t *testing.T) {
	first := uuid.NewString()
	second := uuid.NewString()

	tests := []struct {
		name    string
		value   interface{}
		limit   int
		wantLen int
		wantErr bool
	}{
		{name: "reference ids", value: []interface{}{first, second}, limit: 2, wantLen: 2},
		{name: "objects", value: []interface{}{map[string]interface{}{"name": "one"}, map[string]interface{}{"name": "two"}}, limit: 2, wantLen: 2},
		{name: "json encoded array", value: `["` + first + `","` + second + `"]`, limit: 2, wantLen: 2},
		{name: "empty", value: []interface{}{}, limit: 2, wantLen: 0},
		{name: "not an array", value: map[string]interface{}{}, limit: 2, wantErr: true},
		{name: "invalid json", value: `[`, limit: 2, wantErr: true},
		{name: "json object", value: `{}`, limit: 2, wantErr: true},
		{name: "over limit", value: []interface{}{first, second}, limit: 1, wantErr: true},
		{name: "null item", value: []interface{}{nil}, limit: 1, wantErr: true},
		{name: "invalid reference", value: []interface{}{"not-a-reference"}, limit: 1, wantErr: true},
		{name: "unsupported scalar", value: []interface{}{float64(1)}, limit: 1, wantErr: true},
		{name: "duplicate reference", value: []interface{}{first, first}, limit: 2, wantErr: true},
		{name: "duplicate object reference", value: []interface{}{map[string]interface{}{"reference_id": first, "name": "one"}, map[string]interface{}{"reference_id": first, "name": "two"}}, limit: 2, wantErr: true},
		{name: "duplicate object", value: []interface{}{map[string]interface{}{"name": "same", "enabled": true}, map[string]interface{}{"enabled": true, "name": "same"}}, limit: 2, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			items, err := normalizeActionForeachItems(test.value, test.limit)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeActionForeachItems(%#v) succeeded, want error", test.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeActionForeachItems(%#v): %v", test.value, err)
			}
			if len(items) != test.wantLen {
				t.Fatalf("items = %d, want %d", len(items), test.wantLen)
			}
		})
	}
}

func TestValidateActionForeachOutcome(t *testing.T) {
	valid := actionresponse.Outcome{Type: "asset", Method: "PATCH", ForEach: "~asset_ids", MaxItems: 10}
	if err := validateActionForeachOutcome(valid); err != nil {
		t.Fatalf("valid foreach outcome rejected: %v", err)
	}

	tests := []actionresponse.Outcome{
		{Type: "asset", Method: "PATCH", ForEach: "~asset_ids"},
		{Type: "asset", Method: "PATCH", ForEach: "~asset_ids", MaxItems: maxActionForeachItems + 1},
		{Type: "asset", Method: "GET", ForEach: "~asset_ids", MaxItems: 10},
		{Type: "asset", Method: "PATCH", ForEach: "~asset_ids", MaxItems: 10, ContinueOnError: true},
	}
	for _, outcome := range tests {
		if err := validateActionForeachOutcome(outcome); err == nil {
			t.Fatalf("invalid foreach outcome accepted: %#v", outcome)
		}
	}

	ordinary := actionresponse.Outcome{Type: "asset", Method: "GET", ContinueOnError: true}
	if err := validateActionForeachOutcome(ordinary); err != nil {
		t.Fatalf("ordinary outcome changed by foreach validation: %v", err)
	}
}

func TestForeachItemContextIsIsolated(t *testing.T) {
	base := map[string]interface{}{"stable": "value"}
	first := copyActionContext(base)
	first["item"] = map[string]interface{}{"name": "one"}
	first["item_index"] = 0

	second := copyActionContext(base)
	second["item"] = map[string]interface{}{"name": "two"}
	second["item_index"] = 1

	if _, ok := base["item"]; ok {
		t.Fatal("iteration item leaked into the action context")
	}
	value, err := EvaluateString("~item.name", second)
	if err != nil {
		t.Fatal(err)
	}
	if value != "two" {
		t.Fatalf("item expression = %#v, want two", value)
	}
	if first["item_index"] != 0 || second["item_index"] != 1 {
		t.Fatalf("iteration indexes changed: %#v %#v", first["item_index"], second["item_index"])
	}
}

func TestActionOutcomeConditionSemanticsRemainUnchanged(t *testing.T) {
	tests := []struct {
		condition string
		want      bool
	}{
		{condition: "1", want: true},
		{condition: " 1 ", want: false},
		{condition: "TRUE", want: true},
		{condition: " true ", want: true},
		{condition: "false", want: false},
	}
	for _, test := range tests {
		got, err := shouldExecuteActionOutcome(actionresponse.Action{}, actionresponse.Outcome{Condition: test.condition}, map[string]interface{}{})
		if err != nil {
			t.Fatalf("condition %q: %v", test.condition, err)
		}
		if got != test.want {
			t.Fatalf("condition %q = %t, want %t", test.condition, got, test.want)
		}
	}
}

func TestFinalizeActionOutcomeUsesExistingResponseAndReferenceSemantics(t *testing.T) {
	referenceID := daptinid.DaptinReferenceId(uuid.New())
	visibleAttributes := map[string]interface{}{"reference_id": referenceID}
	context := map[string]interface{}{}
	responses := finalizeActionOutcome(
		actionresponse.ActionRequest{},
		actionresponse.Outcome{Reference: "visible"},
		&auth.SessionUser{}, nil,
		[]actionresponse.ActionResponse{NewActionResponse("asset", visibleAttributes)}, nil, context,
	)
	if len(responses) != 1 || visibleAttributes["reference_id"] != referenceID.String() {
		t.Fatalf("visible response was not finalized using existing semantics: %#v", responses)
	}
	visibleReference, ok := context["visible"].([]interface{})
	if !ok || len(visibleReference) != 1 {
		t.Fatalf("visible reference = %#v", context["visible"])
	}
	referencedAttributes, attributesOK := visibleReference[0].(map[string]interface{})
	if !attributesOK || referencedAttributes["reference_id"] != referenceID.String() {
		t.Fatalf("visible referenced attributes = %#v", visibleReference[0])
	}

	hiddenReferenceID := daptinid.DaptinReferenceId(uuid.New())
	hiddenAttributes := map[string]interface{}{"reference_id": hiddenReferenceID}
	responses = finalizeActionOutcome(
		actionresponse.ActionRequest{},
		actionresponse.Outcome{Reference: "hidden", SkipInResponse: true},
		&auth.SessionUser{}, responses,
		[]actionresponse.ActionResponse{NewActionResponse("asset", hiddenAttributes)}, nil, context,
	)
	if len(responses) != 1 || hiddenAttributes["reference_id"] != hiddenReferenceID {
		t.Fatalf("hidden response behavior changed: responses=%#v attributes=%#v", responses, hiddenAttributes)
	}
}

func TestActionForeachSchemaRoundTrip(t *testing.T) {
	ordinaryEncoded, err := json.Marshal(actionresponse.Outcome{Type: "asset", Method: "PATCH"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ordinaryEncoded, []byte("ForEach")) || bytes.Contains(ordinaryEncoded, []byte("MaxItems")) {
		t.Fatalf("ordinary outcome schema changed shape: %s", ordinaryEncoded)
	}

	action := actionresponse.Action{OutFields: []actionresponse.Outcome{{
		Type: "asset", Method: "PATCH", ForEach: "~asset_ids", MaxItems: 25,
		Attributes: map[string]interface{}{"reference_id": "~item"},
	}}}
	encoded, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	var decoded actionresponse.Action
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.OutFields) != 1 || decoded.OutFields[0].ForEach != "~asset_ids" || decoded.OutFields[0].MaxItems != 25 {
		t.Fatalf("foreach schema did not round trip: %#v", decoded.OutFields)
	}
}

func TestExecuteActionForeachOutcomeCommitsAndRollsBackAsOneTransaction(t *testing.T) {
	database, err := sqlx.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if _, err := database.Exec(`create table asset (
		id integer primary key,
		reference_id blob not null unique,
		name text,
		permission integer,
		version integer,
		updated_at timestamp
	)`); err != nil {
		t.Fatal(err)
	}

	first := daptinid.DaptinReferenceId(uuid.New())
	second := daptinid.DaptinReferenceId(uuid.New())
	for index, referenceID := range []daptinid.DaptinReferenceId{first, second} {
		if _, err := database.Exec(
			`insert into asset (id, reference_id, name, permission, version, updated_at) values (?, ?, ?, ?, ?, ?)`,
			index+1, referenceID[:], "old", int64(auth.DEFAULT_PERMISSION), 1, time.Now(),
		); err != nil {
			t.Fatal(err)
		}
	}

	columns := []api2go.ColumnInfo{
		{Name: "name", ColumnName: "name", ColumnType: "label"},
		{Name: "permission", ColumnName: "permission"},
		{Name: "reference_id", ColumnName: "reference_id"},
		{Name: "version", ColumnName: "version"},
		{Name: "updated_at", ColumnName: "updated_at"},
	}
	adminGroup := daptinid.DaptinReferenceId(uuid.New())
	assetCRUD := &DbResource{
		model: api2go.NewApi2GoModel("asset", columns, int64(auth.DEFAULT_PERMISSION), nil),
		tableInfo: &table_info.TableInfo{
			TableName: "asset", Columns: columns, DefaultPermission: auth.DEFAULT_PERMISSION,
		},
		connection:           database,
		ms:                   &MiddlewareSet{},
		AdministratorGroupId: adminGroup,
	}
	cruds := map[string]*DbResource{"asset": assetCRUD}
	assetCRUD.Cruds = cruds
	oldUserAccountCRUD := CRUD_MAP[USER_ACCOUNT_TABLE_NAME]
	CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = &DbResource{AdministratorGroupId: adminGroup}
	defer func() {
		if oldUserAccountCRUD == nil {
			delete(CRUD_MAP, USER_ACCOUNT_TABLE_NAME)
			return
		}
		CRUD_MAP[USER_ACCOUNT_TABLE_NAME] = oldUserAccountCRUD
	}()

	outcome := actionresponse.Outcome{
		Type: "asset", Method: "PATCH", ForEach: "~updates", MaxItems: 3,
		Attributes: map[string]interface{}{
			"reference_id": "~item.reference_id",
			"name":         "~item.name",
		},
	}
	action := actionresponse.Action{Name: "update_assets", OnType: "document"}
	sessionUser := &auth.SessionUser{Groups: auth.GroupPermissionList{{GroupReferenceId: adminGroup}}}

	successTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	responses, err := assetCRUD.executeActionForeachOutcome(action, outcome, map[string]interface{}{
		"updates": []interface{}{
			map[string]interface{}{"reference_id": first.String(), "name": "first"},
			map[string]interface{}{"reference_id": second.String(), "name": "second"},
		},
	}, sessionUser, successTx)
	if err != nil {
		_ = successTx.Rollback()
		t.Fatalf("execute successful foreach: %v", err)
	}
	if len(responses) != 2 {
		_ = successTx.Rollback()
		t.Fatalf("foreach results = %d, want 2", len(responses))
	}
	if err := successTx.Commit(); err != nil {
		t.Fatal(err)
	}
	assertActionForeachAssetName(t, database, first, "first")
	assertActionForeachAssetName(t, database, second, "second")

	if _, err := database.Exec(`update asset set name = 'old'`); err != nil {
		t.Fatal(err)
	}
	failureTx, err := database.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	missing := uuid.NewString()
	_, err = assetCRUD.executeActionForeachOutcome(action, outcome, map[string]interface{}{
		"updates": []interface{}{
			map[string]interface{}{"reference_id": first.String(), "name": "changed-before-failure"},
			map[string]interface{}{"reference_id": missing, "name": "missing"},
		},
	}, sessionUser, failureTx)
	if err == nil {
		_ = failureTx.Rollback()
		t.Fatal("foreach with a missing second asset succeeded")
	}
	if err := failureTx.Rollback(); err != nil {
		t.Fatal(err)
	}
	assertActionForeachAssetName(t, database, first, "old")
	assertActionForeachAssetName(t, database, second, "old")
}

func assertActionForeachAssetName(t *testing.T, database *sqlx.DB, referenceID daptinid.DaptinReferenceId, want string) {
	t.Helper()
	var got string
	if err := database.QueryRow(`select name from asset where reference_id = ?`, referenceID[:]).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("asset %s name = %q, want %q", referenceID, got, want)
	}
}
