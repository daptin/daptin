package main

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestUsergroupRelationValidationRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: usergroupRelationValidationSchema})
	waitForQueryFilterE2EResource(t, fixture.URL, fixture.Process)
	admin := fixture.SignupAdmin(t)
	probeID := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "filter_probe", map[string]interface{}{"name": "relation-probe"})
	groupID := accessGroupsE2ECreateUsergroup(t, fixture.Client, fixture.URL, admin, "relation-probe-group")
	parentURL := fixture.URL + "/api/filter_probe/" + probeID
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, parentURL+"/relationships/usergroup_id", admin,
		map[string]interface{}{"data": []map[string]interface{}{{"type": "usergroup", "id": groupID}}}, http.StatusNoContent)
	linked := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		parentURL+"/usergroup_id?page%5Bsize%5D=10", admin, nil, http.StatusOK))
	if len(linked) != 1 {
		t.Fatalf("expected one group link, got %#v", linked)
	}
	joinID := linked[0].(map[string]interface{})["id"]
	if joinID == groupID {
		t.Fatal("related-resource response lost join identity")
	}

	requestURL := func(table string, params url.Values) string {
		return fixture.URL + "/api/" + table + "?" + params.Encode()
	}
	invalid := []url.Values{
		{"id`,(SELECT password FROM user_account LIMIT 1)-- Name": {"usergroup_id"}, "a": {"1"}, "b": {"2"}},
		{"unknownName": {"usergroup_id"}, "unknown_id": {probeID}},
		{"filter_probe`Name": {"usergroup_id"}, "filter_probe_id": {probeID}},
		{"filter_probeName": {"usergroup_id"}, "page[size]": {"10"}},
		{"filter_probeName": {"usergroup_id,usergroup_id"}, "filter_probe_id": {probeID}},
		{"filter_probeName": {"usergroup_id"}, "filter_probe_id": {probeID}, "user_accountName": {"usergroup_id"}, "user_account_id": {"mine"}},
	}
	for i, params := range invalid {
		for _, token := range []string{"", admin} {
			t.Run(fmt.Sprintf("invalid_%d_authenticated_%t", i, token != ""), func(t *testing.T) {
				response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, requestURL("usergroup", params), token, nil, http.StatusBadRequest)
				assertConstraintE2EError(t, response, "invalid relation filter", "SELECT password")
			})
		}
	}
	for _, test := range []struct {
		table     string
		params    url.Values
		relatedID string
	}{
		{"usergroup", url.Values{"filter_probeName": {"usergroup_id"}, "filter_probe_id": {probeID}}, groupID},
		{"filter_probe", url.Values{"usergroupName": {"filter_probe_id"}, "usergroup_id": {groupID}}, probeID},
	} {
		for _, paginated := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_paginated_%t", test.table, paginated), func(t *testing.T) {
				params := url.Values{}
				for key, values := range test.params {
					params[key] = values
				}
				if paginated {
					params.Set("page[size]", "10")
				}
				response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, requestURL(test.table, params), admin, nil, http.StatusOK)
				rows := accessGroupsE2EDataArray(t, response)
				if len(rows) != 1 {
					t.Fatalf("expected one related row, got %#v", response)
				}
				row := rows[0].(map[string]interface{})
				attrs := row["attributes"].(map[string]interface{})
				if row["id"] != joinID || attrs["relation_reference_id"] != test.relatedID || attrs["permission"] != float64(32768) {
					t.Fatalf("relationship identity/permission changed: %#v", row)
				}
			})
		}
	}
	// Both linkage and related-resource routes must use the same validated path.
	for _, route := range []struct {
		suffix string
		id     interface{}
	}{
		{"/relationships/usergroup_id", groupID}, {"/usergroup_id", joinID},
	} {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
			parentURL+route.suffix+"?page%5Bsize%5D=10", admin, nil, http.StatusOK)
		rows := accessGroupsE2EDataArray(t, response)
		if len(rows) != 1 || rows[0].(map[string]interface{})["id"] != route.id {
			t.Fatalf("unexpected relationship response: %#v", response)
		}
	}
	// Resolving a valid selector must retain the parent resource's read check.
	reverseURL := requestURL("usergroup", url.Values{"filter_probeName": {"usergroup_id"}, "filter_probe_id": {probeID}})
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, reverseURL, "", nil, http.StatusOK)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, parentURL, admin,
		accessGroupsE2ERecordPayload("filter_probe", probeID, map[string]interface{}{"permission": 0}), http.StatusOK)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, reverseURL, "", nil, http.StatusForbidden)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, reverseURL, admin, nil, http.StatusOK)

}

const usergroupRelationValidationSchema = `
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
  - TableName: filter_probe_filter_probe_id_has_usergroup_usergroup_id
    DefaultPermission: 32768
`
