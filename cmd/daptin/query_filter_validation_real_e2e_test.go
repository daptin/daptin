package main

import (
	"net/http"
	"net/url"
	"testing"
	"time"
)

const queryFilterValidationRealE2ESchema = `
Tables:
  - TableName: filter_probe
    IsTopLevel: true
    Permission: 2097151
    DefaultPermission: 2097151
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
        IsNullable: false

Actions:
  - Name: relation_filter_lookup
    OnType: filter_probe
    InstanceOptional: true
    Permission: 0
    AccessGroups:
      - Name: users
        Permission: 524288
    InFields:
      - Name: relation_filter
        ColumnName: relation_filter
        ColumnType: label
        IsNullable: false
    OutFields:
      - Type: filter_probe
        Method: GET
        Attributes:
          usergroup_id: ~relation_filter
`

func TestQueryFilterValidationRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: queryFilterValidationRealE2ESchema})
	baseURL, client, process := fixture.URL, fixture.Client, fixture.Process
	waitForQueryFilterE2EResource(t, baseURL, process)
	adminToken := fixture.SignupAdmin(t)
	resourceURL := baseURL + "/api/filter_probe"

	validQuery := `[{"column":"name","operator":"is","value":"value"}]`
	accessGroupsE2ERequestJSON(t, client, http.MethodGet, queryFilterE2EURL(resourceURL, validQuery), adminToken, nil, http.StatusOK)

	invalidQueries := []struct {
		name  string
		query string
	}{
		{name: "ordinary", query: `[{"column":"does_not_exist","operator":"is","value":"value"}]`},
		{name: "logical group", query: `[{"column":"name","operator":"is","value":"value","logical_group":"group1"},{"column":"does_not_exist","operator":"is","value":"value","logical_group":"group1"}]`},
		{name: "fuzzy", query: `[{"column":"does_not_exist","operator":"fuzzy","value":"value"}]`},
		{name: "mixed fuzzy columns", query: `[{"column":"name,does_not_exist","operator":"fuzzy","value":"value"}]`},
	}

	for _, test := range invalidQueries {
		t.Run(test.name, func(t *testing.T) {
			response := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
				queryFilterE2EURL(resourceURL, test.query), adminToken, nil, http.StatusBadRequest)
			assertConstraintE2EError(t, response, invalidQueryFilterE2EMessage, "does_not_exist")
		})
	}
}

func TestRelationJoinFilterValidationRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: queryFilterValidationRealE2ESchema})
	waitForQueryFilterE2EResource(t, fixture.URL, fixture.Process)
	adminToken := fixture.SignupAdmin(t)
	userToken := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, adminToken, "relation-filter-user")
	probeID := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, adminToken, "filter_probe", map[string]interface{}{"name": "visible"})
	groupID := accessGroupsE2ECreateUsergroup(t, fixture.Client, fixture.URL, adminToken, "relation-filter-group")
	accessGroupsE2ECreateJoin(t, fixture.Client, fixture.URL, adminToken,
		"filter_probe_filter_probe_id_has_usergroup_usergroup_id",
		map[string]interface{}{"filter_probe_id": probeID, "usergroup_id": groupID}, 32768)

	resourceURL := fixture.URL + "/api/filter_probe"
	filterURL := func(value string) string {
		query := url.Values{}
		query.Set("usergroup_id", value)
		return resourceURL + "?" + query.Encode()
	}

	valid := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		filterURL(groupID+"@(permission:32768)"), userToken, nil, http.StatusOK)
	if got := len(accessGroupsE2EDataArray(t, valid)); got != 1 {
		t.Fatalf("valid relation filter returned %d rows, want 1: %#v", got, valid)
	}
	unmatched := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		filterURL(groupID+"@(permission:0)"), userToken, nil, http.StatusOK)
	if got := len(accessGroupsE2EDataArray(t, unmatched)); got != 0 {
		t.Fatalf("unmatched relation filter returned %d rows, want 0: %#v", got, unmatched)
	}

	invalid := []struct {
		name  string
		value string
	}{
		{"unknown column", groupID + "@(missing:1)"},
		{"unknown column with missing reference", "00000000-0000-0000-0000-000000000000@(missing:1)"},
		{"backtick injection", groupID + "@(permission`<>-1 AND ((SELECT count(*) FROM user_account)>0) OR `name:x)"},
		{"double quote injection", groupID + "@(permission\" OR (SELECT count(*) FROM user_account)>0 OR \"name:x)"},
		{"missing wrapper", groupID + "@permission:32768"},
		{"missing separator", groupID + "@(permission)"},
		{"empty condition", groupID + "@()"},
		{"invalid reference", "not-a-uuid@(permission:32768)"},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
				filterURL(test.value), userToken, nil, http.StatusBadRequest)
		})
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		filterURL(groupID+"@(permission` OR 1=1 OR `name:x)"), "", nil, http.StatusBadRequest)

	actionURL := fixture.URL + "/action/filter_probe/relation_filter_lookup"
	validAction := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, actionURL,
		userToken, map[string]interface{}{"attributes": map[string]interface{}{
			"relation_filter": groupID + "@(permission:32768)",
		}}, http.StatusOK)
	invalidAction := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, actionURL,
		userToken, map[string]interface{}{"attributes": map[string]interface{}{
			"relation_filter": groupID + "@(permission` OR 1=1 OR `name:x)",
		}}, http.StatusOK)
	if got := relationFilterActionRowCount(t, validAction); got != 1 {
		t.Fatalf("valid action relation filter returned %d rows, want 1: %#v", got, validAction)
	}
	if got := relationFilterActionRowCount(t, invalidAction); got != 0 {
		t.Fatalf("injected action relation filter returned %d rows, want 0: %#v", got, invalidAction)
	}
}

func relationFilterActionRowCount(t *testing.T, response interface{}) int {
	t.Helper()
	responses, ok := response.([]interface{})
	if !ok || len(responses) != 1 {
		t.Fatalf("expected one action response, got %#v", response)
	}
	result, ok := responses[0].(map[string]interface{})
	if !ok || result["ResponseType"] != "filter_probe" {
		t.Fatalf("unexpected action response: %#v", response)
	}
	rows, ok := result["Attributes"].([]interface{})
	if !ok {
		t.Fatalf("action response has no row list: %#v", response)
	}
	return len(rows)
}

const invalidQueryFilterE2EMessage = "invalid query filter column"

func queryFilterE2EURL(resourceURL, query string) string {
	return resourceURL + "?query=" + url.QueryEscape(query)
}

func waitForQueryFilterE2EResource(t *testing.T, baseURL string, process *transportE2EDaptinProcess) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(baseURL + "/api/filter_probe")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("filter_probe resource was not registered\n%s", process.logs.String())
}
