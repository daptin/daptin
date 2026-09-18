package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const jsonColumnsRealE2ESchema = `
EnableGraphQL: true

Tables:
  - TableName: json_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
        IsNullable: false
      - Name: metadata
        DataType: text
        ColumnType: json
        IsNullable: true
    Validations:
      - ColumnName: name
        Tags: required

Actions:
  - Name: create_json_probe
    Label: Create JSON probe
    OnType: json_probe
    InstanceOptional: true
    InFields:
      - Name: name
        ColumnName: name
        ColumnType: label
        IsNullable: false
      - Name: metadata
        ColumnName: metadata
        ColumnType: json
        IsNullable: false
    OutFields:
      - Type: json_probe
        Method: POST
        Reference: created
        Attributes:
          name: "~name"
          metadata: "~metadata"
`

func TestJSONColumnsRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the JSON-column e2e")
	}

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	process := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "json-columns.db"), schema: jsonColumnsRealE2ESchema,
	})
	defer process.stopProcess()
	waitForJSONColumnE2EResource(t, baseURL, process)

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	resourceURL := baseURL + "/api/json_probe"

	object := map[string]interface{}{"source": "rest", "rank": float64(1)}
	created := accessGroupsE2ERequestJSON(t, client, http.MethodPost, resourceURL, adminToken,
		accessGroupsE2ERecordPayload("json_probe", "", map[string]interface{}{"name": "object", "metadata": object}), http.StatusCreated)
	assertJSONColumnE2EAttribute(t, created, object)
	createdID := accessGroupsE2EResourceID(t, created)

	fetched := accessGroupsE2ERequestJSON(t, client, http.MethodGet, resourceURL+"/"+createdID, adminToken, nil, http.StatusOK)
	assertJSONColumnE2EAttribute(t, fetched, object)
	encodedObject := map[string]interface{}{"source": "encoded", "rank": float64(2)}
	encodedCreated := accessGroupsE2ERequestJSON(t, client, http.MethodPost, resourceURL, adminToken,
		accessGroupsE2ERecordPayload("json_probe", "", map[string]interface{}{"name": "encoded", "metadata": `{"source":"encoded","rank":2}`}), http.StatusCreated)
	assertJSONColumnE2EAttribute(t, encodedCreated, encodedObject)

	array := []interface{}{map[string]interface{}{"id": float64(1)}, "two", true}
	updated := accessGroupsE2ERequestJSON(t, client, http.MethodPatch, resourceURL+"/"+createdID, adminToken,
		accessGroupsE2ERecordPayload("json_probe", createdID, map[string]interface{}{"metadata": array}), http.StatusOK)
	assertJSONColumnE2EAttribute(t, updated, array)

	invalid := accessGroupsE2ERequestJSON(t, client, http.MethodPost, resourceURL, adminToken,
		accessGroupsE2ERecordPayload("json_probe", "", map[string]interface{}{"name": "invalid", "metadata": `{"broken":`}), http.StatusUnprocessableEntity)
	assertConstraintE2EError(t, invalid, "invalid JSON value for metadata", "unexpected end of JSON input")
	invalidUpdate := accessGroupsE2ERequestJSON(t, client, http.MethodPatch, resourceURL+"/"+createdID, adminToken,
		accessGroupsE2ERecordPayload("json_probe", createdID, map[string]interface{}{"metadata": `not-json`}), http.StatusUnprocessableEntity)
	assertConstraintE2EError(t, invalidUpdate, "invalid JSON value for metadata", "invalid character")

	actionObject := map[string]interface{}{"source": "action", "nested": map[string]interface{}{"enabled": true}}
	accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/action/json_probe/create_json_probe", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{"name": "action", "metadata": actionObject}}, http.StatusOK)
	actionID := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "json_probe", "name", "action")
	actionCreated := accessGroupsE2ERequestJSON(t, client, http.MethodGet, resourceURL+"/"+actionID, adminToken, nil, http.StatusOK)
	assertJSONColumnE2EAttribute(t, actionCreated, actionObject)
	actionInvalid := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/action/json_probe/create_json_probe", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{"name": "invalid action", "metadata": `not-json`}}, http.StatusUnprocessableEntity)
	assertConstraintE2EPublicMessage(t, actionInvalid, "invalid JSON value for metadata", "invalid character")

	graphQLObject := map[string]interface{}{"source": "graphql", "items": []interface{}{float64(1), float64(2)}}
	graphQL := accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/graphql", adminToken,
		map[string]interface{}{
			"query":     `mutation AddJSONProbe($metadata: JSON!) { addJsonProbe(name: "graphql", metadata: $metadata) { name metadata } }`,
			"variables": map[string]interface{}{"metadata": graphQLObject},
		}, http.StatusOK)
	assertGraphQLJSONColumnE2E(t, graphQL, graphQLObject)
}

func assertJSONColumnE2EAttribute(t *testing.T, response interface{}, want interface{}) {
	t.Helper()
	document, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("response is not an object: %#v", response)
	}
	data, ok := document["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response data is not an object: %#v", response)
	}
	attributes, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("response attributes are not an object: %#v", response)
	}
	if got := attributes["metadata"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("metadata = %#v, want %#v", got, want)
	}
}

func assertGraphQLJSONColumnE2E(t *testing.T, response interface{}, want interface{}) {
	t.Helper()
	document, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("GraphQL response is not an object: %#v", response)
	}
	if errorsValue, exists := document["errors"]; exists {
		t.Fatalf("GraphQL response contains errors: %#v", errorsValue)
	}
	data, ok := document["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("GraphQL response data is invalid: %#v", response)
	}
	created, ok := data["addJsonProbe"].(map[string]interface{})
	if !ok {
		t.Fatalf("GraphQL mutation result is invalid: %#v", response)
	}
	if got := created["metadata"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("GraphQL metadata = %#v, want %#v", got, want)
	}
}

func waitForJSONColumnE2EResource(t *testing.T, baseURL string, process *transportE2EDaptinProcess) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/api/json_probe")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("json_probe resource was not registered\n%s", process.logs.String())
}
