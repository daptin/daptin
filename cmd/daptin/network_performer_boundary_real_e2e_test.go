package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// GHSA-h33q-vvvh-vhm4 assumes performer registration creates a public action.
func TestNetworkPerformerBoundaryRealE2E(t *testing.T) {
	var configuredCalls, injectedCalls atomic.Int64
	configured := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configuredCalls.Add(1)
		if r.Method != http.MethodGet || r.Header.Get("X-Injected") != "" || r.URL.RawQuery != "" {
			t.Errorf("caller changed configured request: %s %s %v", r.Method, r.URL, r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"source":"configured destination"}`)
	}))
	defer configured.Close()
	injected := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		injectedCalls.Add(1)
		fmt.Fprint(w, "caller destination")
	}))
	defer injected.Close()

	schema := fmt.Sprintf(`
Tables:
  - TableName: network_boundary
    Permission: 32
    Columns:
      - Name: name
        DataType: varchar(100)
        ColumnType: label
Actions:
  - Name: configured_request
    OnType: network_boundary
    InstanceOptional: true
    Permission: 32
    OutFields:
      - Type: $network.request
        Method: EXECUTE
        Attributes:
          Url: %q
          Method: GET
  - Name: restricted_request
    OnType: network_boundary
    InstanceOptional: true
    Permission: 0
    OutFields:
      - Type: $network.request
        Method: EXECUTE
        Attributes:
          Url: %q
          Method: GET
`, configured.URL, configured.URL)
	f := startDaptinE2E(t, transportE2EDaptinOptions{schema: schema})
	admin := f.SignupAdmin(t)
	user := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "network-observer")
	payload := map[string]interface{}{
		"Url": injected.URL, "Method": "DELETE",
		"Headers": map[string]interface{}{"X-Injected": "yes"},
		"Query":   map[string]interface{}{"injected": "yes"},
		"OutFields": []interface{}{map[string]interface{}{
			"Type": "$network.request", "Method": "EXECUTE",
			"Attributes": map[string]interface{}{"Url": injected.URL},
		}},
	}
	for _, token := range []string{"", user, admin} {
		for _, entity := range []string{"cloud_store", "world", "unregistered_type"} {
			for _, method := range []string{http.MethodPost, http.MethodGet, http.MethodPut, http.MethodPatch, http.MethodDelete} {
				accessGroupsE2EAssertStatus(t, f.Client, method,
					f.URL+"/action/"+entity+"/$network.request", token, payload, http.StatusNotFound)
			}
		}
	}
	for _, token := range []string{"", user} {
		accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost,
			f.URL+"/action/network_boundary/restricted_request", token, payload, http.StatusForbidden)
	}
	if configuredCalls.Load() != 0 || injectedCalls.Load() != 0 {
		t.Fatal("missing or forbidden action reached a network destination")
	}
	for _, token := range []string{"", user, admin} {
		response := accessGroupsE2ERequestJSON(t, f.Client, http.MethodPost,
			f.URL+"/action/network_boundary/configured_request", token, payload, http.StatusOK)
		if source, ok := transportE2EFindString(response, "source"); !ok || source != "configured destination" {
			t.Fatalf("configured action response = %#v", response)
		}
	}
	accessGroupsE2EAssertStatus(t, f.Client, http.MethodPost,
		f.URL+"/action/network_boundary/restricted_request", admin, payload, http.StatusOK)
	if configuredCalls.Load() != 4 || injectedCalls.Load() != 0 {
		t.Fatalf("network calls: configured=%d, injected=%d", configuredCalls.Load(), injectedCalls.Load())
	}
}
