package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"
)

const actionForeachE2ESchema = `
EnableGraphQL: true

Tables:
  - TableName: foreach_document
    Permission: 786432
    DefaultPermission: 1280
    AccessGroups:
      - Name: users
        Permission: 786432
    Columns:
      - Name: title
        DataType: varchar(100)
        ColumnType: label

  - TableName: foreach_asset
    Permission: 262144
    DefaultPermission: 1280
    AccessGroups:
      - Name: users
        Permission: 262144
    Columns:
      - Name: state
        DataType: varchar(100)
        ColumnType: label
    Validations:
      - ColumnName: state
        Tags: oneof=old first second

Actions:
  - Name: route_echo
    Label: Route echo
    OnType: foreach_document
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    OutFields:
      - Type: foreach.route
        Method: ACTIONRESPONSE
        Attributes:
          status: ok

  - Name: update_manifest
    Label: Update manifest
    OnType: foreach_document
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    InFields:
      - Name: document_id
        ColumnName: document_id
        ColumnType: label
        IsNullable: false
      - Name: title
        ColumnName: title
        ColumnType: label
        IsNullable: false
      - Name: asset_updates
        ColumnName: asset_updates
        ColumnType: json
        IsNullable: false
    OutFields:
      - Type: foreach_document
        Method: PATCH
        Attributes:
          reference_id: ~document_id
          title: ~title
      - Type: foreach_asset
        Method: PATCH
        ForEach: ~asset_updates
        MaxItems: 3
        Reference: updated_assets
        Attributes:
          reference_id: ~item.reference_id
          state: ~item.state
      - Type: foreach.summary
        Method: ACTIONRESPONSE
        Attributes:
          first_state: $updated_assets[0].state

  - Name: ordinary_crud_round_trip
    Label: Ordinary CRUD round trip
    OnType: foreach_document
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    OutFields:
      - Type: foreach_asset
        Method: POST
        Reference: created_asset
        Attributes:
          state: first
      - Type: foreach_asset
        Method: PATCH
        Reference: updated_asset
        Attributes:
          reference_id: $created_asset.reference_id
          state: second
      - Type: foreach_asset
        Method: DELETE
        Attributes:
          reference_id: $updated_asset.reference_id

  - Name: create_assets
    Label: Create assets
    OnType: foreach_document
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    InFields:
      - Name: asset_states
        ColumnName: asset_states
        ColumnType: json
        IsNullable: false
    OutFields:
      - Type: foreach_asset
        Method: POST
        ForEach: ~asset_states
        MaxItems: 3
        Attributes:
          state: ~item.state

  - Name: delete_assets
    Label: Delete assets
    OnType: foreach_document
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    InFields:
      - Name: asset_ids
        ColumnName: asset_ids
        ColumnType: json
        IsNullable: false
    OutFields:
      - Type: foreach_asset
        Method: DELETE
        ForEach: ~asset_ids
        MaxItems: 3
        Attributes:
          reference_id: ~item

  - Name: conditionally_update_assets
    Label: Conditionally update assets
    OnType: foreach_document
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    InFields:
      - Name: asset_updates
        ColumnName: asset_updates
        ColumnType: json
        IsNullable: false
    OutFields:
      - Type: foreach_asset
        Method: PATCH
        ForEach: ~asset_updates
        MaxItems: 3
        Condition: "!item.apply === true && item_index === 0"
        Attributes:
          reference_id: ~item.reference_id
          state: ~item.state
`

func TestTransactionalActionForeachRealE2E(t *testing.T) {
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the transactional action foreach e2e")
	}

	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{schema: actionForeachE2ESchema})
	defer daptinProcess.stopProcess()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	userToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "foreach-user")
	for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodPatch, http.MethodPut, http.MethodDelete} {
		routeResponse := accessGroupsE2ERequestJSON(t, client, method,
			baseURL+"/action/foreach_document/route_echo", userToken,
			map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
		routeResponses, ok := routeResponse.([]interface{})
		if !ok || len(routeResponses) != 1 {
			t.Fatalf("%s action route response = %#v", method, routeResponse)
		}
		assertActionForeachE2EResponseAttribute(t, routeResponses[0], "status", "ok")
	}
	updateActionID := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "action", "action_name", "update_manifest")
	actionSchemaResponse := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/action/get_action_schema", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{"action_id": updateActionID}}, http.StatusOK)
	actionSchema := actionForeachE2EDownloadContent(t, actionSchemaResponse)
	if actionSchema["OutFields"] == nil {
		t.Fatalf("downloaded action schema has no OutFields: %#v", actionSchema)
	}
	actionSchemaJSON, err := json.Marshal(actionSchema["OutFields"])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(actionSchemaJSON, []byte(`"ForEach":"~asset_updates"`)) ||
		!bytes.Contains(actionSchemaJSON, []byte(`"MaxItems":3`)) {
		t.Fatalf("downloaded action schema does not expose ForEach configuration: %s", actionSchemaJSON)
	}

	documentID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "foreach_document", map[string]interface{}{"title": "old"})
	firstAssetID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "foreach_asset", map[string]interface{}{"state": "old"})
	secondAssetID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "foreach_asset", map[string]interface{}{"state": "old"})

	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/foreach_asset/"+firstAssetID, userToken,
		accessGroupsE2ERecordPayload("foreach_asset", firstAssetID, map[string]interface{}{"state": "first"}), http.StatusForbidden)

	actionURL := baseURL + "/action/foreach_document/update_manifest"
	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, actionURL, userToken, map[string]interface{}{
		"attributes": map[string]interface{}{
			"document_id": documentID,
			"title":       "committed",
			"asset_updates": []interface{}{
				map[string]interface{}{"reference_id": firstAssetID, "state": "first"},
				map[string]interface{}{"reference_id": secondAssetID, "state": "second"},
			},
		},
	}, http.StatusOK)
	responseArray, ok := response.([]interface{})
	if !ok || len(responseArray) != 4 {
		t.Fatalf("foreach action response = %#v, want document, two assets, and summary", response)
	}
	assertActionForeachE2EResponseAttribute(t, responseArray[3], "first_state", "first")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_document", documentID, "title", "committed")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", firstAssetID, "state", "first")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", secondAssetID, "state", "second")

	accessGroupsE2EAssertStatus(t, client, http.MethodPost, actionURL, userToken, map[string]interface{}{
		"attributes": map[string]interface{}{
			"document_id": documentID,
			"title":       "must-roll-back",
			"asset_updates": []interface{}{
				map[string]interface{}{"reference_id": firstAssetID, "state": "second"},
				map[string]interface{}{"reference_id": secondAssetID, "state": "invalid"},
			},
		},
	}, http.StatusBadRequest)
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_document", documentID, "title", "committed")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", firstAssetID, "state", "first")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", secondAssetID, "state", "second")

	for name, request := range map[string]struct {
		updates interface{}
		status  int
	}{
		"duplicate": {
			updates: []interface{}{
				map[string]interface{}{"reference_id": firstAssetID, "state": "second"},
				map[string]interface{}{"reference_id": firstAssetID, "state": "first"},
			},
			status: http.StatusBadRequest,
		},
		"over limit": {
			updates: []interface{}{
				map[string]interface{}{"reference_id": firstAssetID, "state": "second"},
				map[string]interface{}{"reference_id": secondAssetID, "state": "first"},
				map[string]interface{}{"reference_id": fmt.Sprintf("00000000-0000-0000-0000-%012d", 1), "state": "first"},
				map[string]interface{}{"reference_id": fmt.Sprintf("00000000-0000-0000-0000-%012d", 2), "state": "second"},
			},
			status: http.StatusRequestEntityTooLarge,
		},
		"not an array": {updates: map[string]interface{}{"reference_id": firstAssetID}, status: http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			accessGroupsE2EAssertStatus(t, client, http.MethodPost, actionURL, userToken, map[string]interface{}{
				"attributes": map[string]interface{}{
					"document_id":   documentID,
					"title":         "must-roll-back-" + name,
					"asset_updates": request.updates,
				},
			}, request.status)
			assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_document", documentID, "title", "committed")
			assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", firstAssetID, "state", "first")
		})
	}

	graphQLUpdates, err := json.Marshal([]interface{}{
		map[string]interface{}{"reference_id": firstAssetID, "state": "second"},
		map[string]interface{}{"reference_id": secondAssetID, "state": "first"},
	})
	if err != nil {
		t.Fatal(err)
	}
	graphQLResponse := transportE2EPostJSON(t, client, baseURL+"/graphql", userToken, map[string]interface{}{
		"query": fmt.Sprintf(`mutation { executeUpdateManifestOnForeachDocument(document_id: %q, title: "graphql", asset_updates: %q) { ResponseType } }`, documentID, string(graphQLUpdates)),
	})
	if transportE2EGraphQLHasErrors(graphQLResponse) {
		t.Fatalf("GraphQL foreach action failed: %#v", graphQLResponse)
	}
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_document", documentID, "title", "graphql")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", firstAssetID, "state", "second")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", secondAssetID, "state", "first")

	conditionalResponse := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/foreach_document/conditionally_update_assets", userToken, map[string]interface{}{
			"attributes": map[string]interface{}{"asset_updates": []interface{}{
				map[string]interface{}{"reference_id": firstAssetID, "state": "old", "apply": true},
				map[string]interface{}{"reference_id": secondAssetID, "state": "second", "apply": true},
			}},
		}, http.StatusOK)
	conditionalResponses, ok := conditionalResponse.([]interface{})
	if !ok || len(conditionalResponses) != 1 {
		t.Fatalf("conditional foreach response = %#v, want one updated asset", conditionalResponse)
	}
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", firstAssetID, "state", "old")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", secondAssetID, "state", "first")

	ordinaryResponse := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/foreach_document/ordinary_crud_round_trip", userToken,
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	ordinaryResponses, ok := ordinaryResponse.([]interface{})
	if !ok || len(ordinaryResponses) != 3 {
		t.Fatalf("ordinary CRUD response = %#v, want POST, PATCH, DELETE responses", ordinaryResponse)
	}
	ordinaryAssetID := actionForeachE2EResponseReferenceID(t, ordinaryResponses[0])
	accessGroupsE2EAssertStatus(t, client, http.MethodGet, baseURL+"/api/foreach_asset/"+ordinaryAssetID, adminToken, nil, http.StatusInternalServerError)

	accessGroupsE2EAssertListCount(t, client, baseURL, adminToken, "foreach_asset", 2)
	accessGroupsE2EAssertStatus(t, client, http.MethodPost,
		baseURL+"/action/foreach_document/create_assets", userToken, map[string]interface{}{
			"attributes": map[string]interface{}{"asset_states": []interface{}{
				map[string]interface{}{"state": "first"},
				map[string]interface{}{"state": "invalid"},
			}},
		}, http.StatusBadRequest)
	accessGroupsE2EAssertListCount(t, client, baseURL, adminToken, "foreach_asset", 2)

	createdResponse := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/foreach_document/create_assets", userToken, map[string]interface{}{
			"attributes": map[string]interface{}{"asset_states": []interface{}{
				map[string]interface{}{"state": "first"},
				map[string]interface{}{"state": "second"},
			}},
		}, http.StatusOK)
	createdResponses, ok := createdResponse.([]interface{})
	if !ok || len(createdResponses) != 2 {
		t.Fatalf("foreach POST response = %#v, want two assets", createdResponse)
	}
	createdIDs := []string{
		actionForeachE2EResponseReferenceID(t, createdResponses[0]),
		actionForeachE2EResponseReferenceID(t, createdResponses[1]),
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPost,
		baseURL+"/action/foreach_document/delete_assets", userToken,
		map[string]interface{}{"attributes": map[string]interface{}{"asset_ids": []string{
			createdIDs[0], "00000000-0000-0000-0000-000000000099",
		}}}, http.StatusInternalServerError)
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", createdIDs[0], "state", "first")
	assertActionForeachE2EAttribute(t, client, baseURL, adminToken, "foreach_asset", createdIDs[1], "state", "second")

	accessGroupsE2EAssertStatus(t, client, http.MethodPost,
		baseURL+"/action/foreach_document/delete_assets", userToken,
		map[string]interface{}{"attributes": map[string]interface{}{"asset_ids": createdIDs}}, http.StatusOK)
	for _, id := range createdIDs {
		accessGroupsE2EAssertStatus(t, client, http.MethodGet, baseURL+"/api/foreach_asset/"+id, adminToken, nil, http.StatusInternalServerError)
	}
}

func assertActionForeachE2EResponseAttribute(t *testing.T, response interface{}, attribute string, want interface{}) {
	t.Helper()
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("action response is not an object: %#v", response)
	}
	attributes, ok := responseMap["Attributes"].(map[string]interface{})
	if !ok || attributes[attribute] != want {
		t.Fatalf("action response %s = %#v, want %#v: %#v", attribute, attributes[attribute], want, response)
	}
}

func actionForeachE2EDownloadContent(t *testing.T, response interface{}) map[string]interface{} {
	t.Helper()
	responses, ok := response.([]interface{})
	if !ok || len(responses) != 1 {
		t.Fatalf("action schema response = %#v", response)
	}
	responseMap, ok := responses[0].(map[string]interface{})
	if !ok {
		t.Fatalf("action schema response is not an object: %#v", response)
	}
	attributes, ok := responseMap["Attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("action schema attributes are not an object: %#v", response)
	}
	encoded, ok := attributes["content"].(string)
	if !ok {
		t.Fatalf("action schema response has no content: %#v", response)
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("decode action schema: %v", err)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(decoded, &schema); err != nil {
		t.Fatalf("unmarshal action schema: %v\n%s", err, decoded)
	}
	return schema
}

func actionForeachE2EResponseReferenceID(t *testing.T, response interface{}) string {
	t.Helper()
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("action response is not an object: %#v", response)
	}
	attributes, ok := responseMap["Attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("action response attributes are not an object: %#v", response)
	}
	referenceID, ok := attributes["reference_id"].(string)
	if !ok || referenceID == "" {
		t.Fatalf("action response has no reference_id: %#v", response)
	}
	return referenceID
}

func assertActionForeachE2EAttribute(t *testing.T, client *http.Client, baseURL, token, entity, id, attribute string, want interface{}) {
	t.Helper()
	response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/"+entity+"/"+id, token, nil, http.StatusOK)
	responseMap, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("%s response is not an object: %#v", entity, response)
	}
	data, ok := responseMap["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("%s response data is not an object: %#v", entity, response)
	}
	attributes, ok := data["attributes"].(map[string]interface{})
	if !ok || attributes[attribute] != want {
		t.Fatalf("%s %s = %#v, want %#v: %#v", entity, attribute, attributes[attribute], want, response)
	}
}
