package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const conformationRealE2ESchema = `
Tables:
  - TableName: conformation_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: code
        DataType: varchar(40)
        ColumnType: alias
      - Name: name
        DataType: varchar(100)
        ColumnType: label
      - Name: contact_email
        DataType: varchar(100)
        ColumnType: email
      - Name: count
        DataType: int(11)
        ColumnType: measurement
    Validations:
      - ColumnName: code
        Tags: required,alphanum
      - ColumnName: name
        Tags: required
    Conformations:
      - ColumnName: code
        Tags: trim,upper
      - ColumnName: name
        Tags: trim,title
      - ColumnName: contact_email
        Tags: trim,email
      - ColumnName: count
        Tags: trim

Actions:
  - Name: create_conformed_probe
    Label: Create conformed probe
    OnType: conformation_probe
    InstanceOptional: true
    InFields:
      - Name: code
        ColumnName: code
        ColumnType: label
        IsNullable: false
    Validations:
      - ColumnName: code
        Tags: required,alphanum
    Conformations:
      - ColumnName: code
        Tags: trim,upper
    OutFields:
      - Type: conformation_probe
        Method: POST
        Reference: created
        Attributes:
          code: "~code"
          name: "  transactional widget  "
          contact_email: "TX@EXAMPLE.COM"
          count: 9
`

func TestDeclaredConformationsRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the declared-conformation e2e")
	}

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	databasePath := filepath.Join(t.TempDir(), "declared-conformations.db")
	options := transportE2EDaptinOptions{
		databaseType: "sqlite3", connectionString: databasePath, schema: conformationRealE2ESchema,
	}
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, options)
	defer func() { daptinProcess.stopProcess() }()
	waitForConformationE2EResource(t, baseURL, daptinProcess)

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	assertConformationE2EWorldRegistered(t, client, baseURL, adminToken, daptinProcess)

	created := conformationE2EPostResource(t, client, baseURL+"/api/conformation_probe", adminToken,
		accessGroupsE2ERecordPayload("conformation_probe", "", map[string]interface{}{
			"code":          "  ab12  ",
			"name":          "  direct widget  ",
			"contact_email": "TEST@EXAMPLE.COM",
			"count":         7,
		}), daptinProcess)
	createdID := accessGroupsE2EResourceID(t, created)
	assertConformationE2EAttributes(t, created, map[string]interface{}{
		"code": "AB12", "name": "Direct Widget", "contact_email": "TEST@example.com", "count": float64(7),
	})

	fetched := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		baseURL+"/api/conformation_probe/"+createdID, adminToken, nil, http.StatusOK)
	assertConformationE2EAttributes(t, fetched, map[string]interface{}{
		"code": "AB12", "name": "Direct Widget", "contact_email": "TEST@example.com", "count": float64(7),
	})

	updated := accessGroupsE2ERequestJSON(t, client, http.MethodPatch,
		baseURL+"/api/conformation_probe/"+createdID, adminToken,
		accessGroupsE2ERecordPayload("conformation_probe", createdID, map[string]interface{}{
			"code": "  cd34  ", "name": "  updated widget  ",
		}), http.StatusOK)
	assertConformationE2EAttributes(t, updated, map[string]interface{}{
		"code": "CD34", "name": "Updated Widget",
	})

	accessGroupsE2EAssertStatus(t, client, http.MethodPost, baseURL+"/api/conformation_probe", adminToken,
		accessGroupsE2ERecordPayload("conformation_probe", "", map[string]interface{}{
			"code": "  invalid-code  ", "name": "Rejected",
		}), http.StatusBadRequest)

	accessGroupsE2EAssertStatus(t, client, http.MethodPost,
		baseURL+"/action/conformation_probe/create_conformed_probe", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{"code": "  tx77  "}}, http.StatusOK)
	actionCreatedID := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "conformation_probe", "code", "TX77")
	actionCreated := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		baseURL+"/api/conformation_probe/"+actionCreatedID, adminToken, nil, http.StatusOK)
	assertConformationE2EAttributes(t, actionCreated, map[string]interface{}{
		"code": "TX77", "name": "Transactional Widget", "contact_email": "TX@example.com", "count": float64(9),
	})
}

func conformationE2EPostResource(t *testing.T, client *http.Client, requestURL string, token string, payload interface{}, process *transportE2EDaptinProcess) interface{} {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/vnd.api+json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	responseBody, _ := io.ReadAll(response.Body)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("POST %s returned %d: %s\n%s", requestURL, response.StatusCode, responseBody, process.logs.String())
	}
	var decoded interface{}
	if err := json.Unmarshal(responseBody, &decoded); err != nil {
		t.Fatalf("decode create response: %v: %s", err, responseBody)
	}
	return decoded
}

func assertConformationE2EWorldRegistered(t *testing.T, client *http.Client, baseURL string, token string, process *transportE2EDaptinProcess) {
	t.Helper()
	response := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		baseURL+"/api/world?page%5Bsize%5D=200", token, nil, http.StatusOK)
	for _, item := range accessGroupsE2EDataArray(t, response) {
		itemMap, _ := item.(map[string]interface{})
		attributes, _ := itemMap["attributes"].(map[string]interface{})
		if attributes["table_name"] == "conformation_probe" {
			return
		}
	}
	t.Fatalf("conformation_probe world was not registered\n%s", process.logs.String())
}

func waitForConformationE2EResource(t *testing.T, baseURL string, process *transportE2EDaptinProcess) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/api/conformation_probe")
		if err == nil {
			body, _ := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				var document map[string]interface{}
				if json.Unmarshal(body, &document) == nil {
					if _, ok := document["data"]; ok {
						return
					}
				}
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("conformation_probe resource was not registered\n%s", process.logs.String())
}

func assertConformationE2EAttributes(t *testing.T, response interface{}, expected map[string]interface{}) {
	t.Helper()
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("response is not an object: %#v", response)
	}
	data, ok := responseMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("response data is not an object: %#v", response)
	}
	attributes, ok := data["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("response attributes are not an object: %#v", response)
	}
	for key, want := range expected {
		if got := attributes[key]; got != want {
			t.Fatalf("attribute %s = %#v, want %#v; response=%#v", key, got, want, response)
		}
	}
}
