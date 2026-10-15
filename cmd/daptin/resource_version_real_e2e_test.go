package main

import (
	"net/http"
	"testing"
)

// The revision read by an API client must be usable as an atomic write precondition.
func TestResourceVersionRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	token := fixture.SignupAdmin(t)
	id := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, token, "document", map[string]interface{}{
		"document_name": "Original", "document_path": "/", "document_extension": "vdocs",
		"mime_type": "application/vnd.vibedada.docs+json;version=3", "document_content": []interface{}{},
	})
	endpoint := fixture.URL + "/api/document/" + id
	read := func() map[string]interface{} {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, endpoint, token, nil, http.StatusOK)
		return response.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	}
	original := read()
	version, ok := original["version"].(float64)
	if !ok {
		t.Fatalf("API did not expose its revision: %#v", original)
	}
	patch := func(name string, expected interface{}, status int) {
		t.Helper()
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, endpoint, token, map[string]interface{}{
			"data": map[string]interface{}{"type": "document", "id": id, "attributes": map[string]interface{}{
				"document_name": name, "version": expected,
			}},
		}, status)
	}
	for _, invalid := range []interface{}{0, -1, 1.5, "1", nil} {
		patch("Invalid revision", invalid, http.StatusUnprocessableEntity)
	}
	patch("Winner", version, http.StatusOK)
	patch("Stale overwrite", version, http.StatusConflict)
	current := read()
	if current["document_name"] != "Winner" || current["version"] != version+1 {
		t.Fatalf("stale write changed the winner or revision: %#v", current)
	}
	patch("Next save", version+1, http.StatusOK)
	if current := read(); current["version"] != version+2 {
		t.Fatalf("next valid save did not advance revision: %#v", current)
	}
}
