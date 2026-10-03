package main

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAggregateSensitiveColumnsRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: "EnableGraphQL: true\n"})
	adminToken := fixture.SignupAdmin(t)
	userToken := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, adminToken, "aggregate-user")

	for _, test := range []struct {
		method  string
		url     string
		payload interface{}
	}{
		{http.MethodGet, fixture.URL + "/aggregate/user_account?group=email&column=email,max(password)", nil},
		{http.MethodPost, fixture.URL + "/aggregate/user_account", map[string]interface{}{
			"group": []string{"email"}, "column": []string{"email", "max(password)"},
		}},
	} {
		accessGroupsE2ERequestJSON(t, fixture.Client, test.method, test.url, userToken, test.payload, http.StatusBadRequest)
	}

	graphQL := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/graphql", userToken,
		map[string]interface{}{"query": `query { aggregateUserAccount(group: ["email"], column: ["email", "max(password)"]) { email } }`},
		http.StatusOK)
	if !transportE2EGraphQLHasErrors(graphQL) || !strings.Contains(fmt.Sprint(graphQL), "invalid aggregation query") {
		t.Fatalf("GraphQL aggregate did not reject password: %#v", graphQL)
	}

	allowed := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		fixture.URL+"/aggregate/user_account?column=count", userToken, nil, http.StatusOK)
	rows := accessGroupsE2EDataArray(t, allowed)
	if len(rows) != 1 {
		t.Fatalf("count aggregate returned %d rows: %#v", len(rows), allowed)
	}
	attributes, ok := rows[0].(map[string]interface{})["attributes"].(map[string]interface{})
	if !ok {
		t.Fatalf("count aggregate has no attributes: %#v", allowed)
	}
	count, ok := attributes["count"].(float64)
	if !ok || count < 2 {
		t.Fatalf("count aggregate did not include multiple accounts: %#v", allowed)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		fixture.URL+"/aggregate/user_account?join=usergroup@eq(user_account.id,usergroup.id)&column=count",
		adminToken, nil, http.StatusOK)
}
