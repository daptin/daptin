package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const constraintErrorsRealE2ESchema = `
Tables:
  - TableName: constraint_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
        IsNullable: false
      - Name: sku
        DataType: varchar(40)
        ColumnType: alias
        IsNullable: false
        IsUnique: true
      - Name: unguarded
        DataType: varchar(40)
        ColumnType: alias
        IsNullable: false
    Validations:
      - ColumnName: name
        Tags: required
      - ColumnName: sku
        Tags: required,alphanum

Actions:
  - Name: create_constraint_probe
    Label: Create constraint probe
    OnType: constraint_probe
    InstanceOptional: true
    InFields:
      - Name: name
        ColumnName: name
        ColumnType: label
        IsNullable: false
      - Name: sku
        ColumnName: sku
        ColumnType: alias
        IsNullable: false
      - Name: unguarded
        ColumnName: unguarded
        ColumnType: alias
        IsNullable: false
    OutFields:
      - Type: constraint_probe
        Method: POST
        Reference: created
        Attributes:
          name: "~name"
          sku: "~sku"
          unguarded: "~unguarded"
`

func TestConstraintErrorsRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the constraint-error e2e")
	}

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "constraint-errors.db"), schema: constraintErrorsRealE2ESchema,
	})
	defer process.stopProcess()
	waitForConstraintE2EResource(t, baseURL, process)

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	resourceURL := baseURL + "/api/constraint_probe"

	missingRequired := accessGroupsE2ERequestJSON(t, client, http.MethodPost, resourceURL, adminToken,
		constraintE2EPayload("", "SKU1", "value"), http.StatusBadRequest)
	assertConstraintE2EError(t, missingRequired, "required", "NOT NULL constraint failed")

	databaseRequired := accessGroupsE2ERequestJSON(t, client, http.MethodPost, resourceURL, adminToken,
		constraintE2EPayload("First", "SKU1", ""), http.StatusUnprocessableEntity)
	assertConstraintE2EError(t, databaseRequired, requiredValueMessageForE2E, "NOT NULL constraint failed")

	conformationE2EPostResource(t, client, resourceURL, adminToken,
		constraintE2EPayload("First", "SKU1", "value"), process)

	duplicate := accessGroupsE2ERequestJSON(t, client, http.MethodPost, resourceURL, adminToken,
		constraintE2EPayload("Duplicate", "SKU1", "value"), http.StatusConflict)
	assertConstraintE2EError(t, duplicate, uniqueConstraintMessageForE2E, "UNIQUE constraint failed")

	second := conformationE2EPostResource(t, client, resourceURL, adminToken,
		constraintE2EPayload("Second", "SKU2", "value"), process)
	secondID := accessGroupsE2EResourceID(t, second)
	patchDuplicate := accessGroupsE2ERequestJSON(t, client, http.MethodPatch, resourceURL+"/"+secondID, adminToken,
		accessGroupsE2ERecordPayload("constraint_probe", secondID, map[string]interface{}{"sku": "SKU1"}), http.StatusConflict)
	assertConstraintE2EError(t, patchDuplicate, uniqueConstraintMessageForE2E, "UNIQUE constraint failed")

	actionDuplicate := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/constraint_probe/create_constraint_probe", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{
			"name": "Action duplicate", "sku": "SKU1", "unguarded": "value",
		}}, http.StatusConflict)
	assertConstraintE2EPublicMessage(t, actionDuplicate, uniqueConstraintMessageForE2E, "UNIQUE constraint failed")
}

const (
	uniqueConstraintMessageForE2E = "unique constraint violated"
	requiredValueMessageForE2E    = "required value missing"
)

func constraintE2EPayload(name string, sku string, unguarded string) map[string]interface{} {
	attributes := map[string]interface{}{"sku": sku}
	if name != "" {
		attributes["name"] = name
	}
	if unguarded != "" {
		attributes["unguarded"] = unguarded
	}
	return accessGroupsE2ERecordPayload("constraint_probe", "", attributes)
}

func assertConstraintE2EError(t *testing.T, response interface{}, wantTitle string, forbidden string) {
	t.Helper()
	document, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("error response is not an object: %#v", response)
	}
	errorsList, ok := document["errors"].([]interface{})
	if !ok || len(errorsList) == 0 {
		t.Fatalf("error response has no JSON:API errors: %#v", response)
	}
	first, ok := errorsList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("first JSON:API error is invalid: %#v", response)
	}
	title, _ := first["title"].(string)
	if !strings.Contains(title, wantTitle) {
		t.Fatalf("error title = %q, want it to contain %q", title, wantTitle)
	}
	assertConstraintE2ENotLeaked(t, response, forbidden)
}

func assertConstraintE2EPublicMessage(t *testing.T, response interface{}, want string, forbidden string) {
	t.Helper()
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), want) {
		t.Fatalf("response %s does not contain public message %q", encoded, want)
	}
	assertConstraintE2ENotLeaked(t, response, forbidden)
}

func assertConstraintE2ENotLeaked(t *testing.T, response interface{}, forbidden string) {
	t.Helper()
	encoded, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), forbidden) {
		t.Fatalf("database detail leaked in response: %s", encoded)
	}
}

func waitForConstraintE2EResource(t *testing.T, baseURL string, process *transportE2EDaptinProcess) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/api/constraint_probe")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("constraint_probe resource was not registered\n%s", process.logs.String())
}
