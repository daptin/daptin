package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

func TestDocumentActionsRealE2E(t *testing.T) {
	requireRealE2E(t)
	// Run the published example itself so action definitions cannot drift from
	// the documented inputs, resource lifecycle or performer return contract.
	schema, err := os.ReadFile("examples/document-processing/schema_document_processing.yaml")
	if err != nil {
		t.Fatal(err)
	}
	options := transportE2EDaptinOptions{schema: string(schema), connectionString: filepath.Join(t.TempDir(), "documents.db")}
	fixture := startDaptinE2E(t, options)
	client, base := fixture.Client, fixture.URL
	admin := fixture.SignupAdmin(t)
	owner := accessGroupsE2ESignupSigninUser(t, client, base, admin, "document-owner")
	other := accessGroupsE2ESignupSigninUser(t, client, base, admin, "document-other")
	ownerID := accessGroupsE2EFindResourceID(t, client, base, admin, "user_account", "email", "document-owner@test.local")
	encoded := base64.StdEncoding.EncodeToString([]byte("Document composition through Daptin"))
	attrs := map[string]interface{}{"source_format": "txt", "file": []interface{}{map[string]interface{}{"name": "source.txt", "contents": encoded}}}
	payload := map[string]interface{}{"attributes": attrs}
	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/conversion_document/extract_document", owner, payload, http.StatusOK)
	text, found := accessGroupsE2EFindString(response, "text")
	if !found || text != "Document composition through Daptin" {
		t.Fatalf("extraction result: %#v", response)
	}
	attrs["target_format"] = "pdf"
	response = accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/conversion_document/convert_document", owner, payload, http.StatusOK)
	assertDocumentE2EPDF(t, response)
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, base+"/action/conversion_document/convert_document", "", payload, http.StatusForbidden)

	documentID := accessGroupsE2ECreateRecord(t, client, base, owner, "conversion_document", map[string]interface{}{
		"title": "Private source", "source_format": "txt", "source_base64": encoded,
	})
	savedPayload := map[string]interface{}{"attributes": map[string]interface{}{"conversion_document_id": documentID}}
	saveURL := base + "/action/conversion_document/save_pdf"
	response = accessGroupsE2ERequestJSON(t, client, http.MethodPost, saveURL, owner, savedPayload, http.StatusOK)
	pdf := assertDocumentE2EPDF(t, response)
	rowURL := base + "/api/conversion_document/" + documentID
	row := accessGroupsE2ERequestJSON(t, client, http.MethodGet, rowURL, owner, nil, http.StatusOK)
	stored, ok := accessGroupsE2EFindString(row, "output_base64")
	if !ok || stored != base64.StdEncoding.EncodeToString(pdf) {
		t.Fatalf("output not persisted through resource PATCH: %#v", row)
	}
	usageID := accessGroupsE2EFindResourceID(t, client, base, admin, "api_usage", "action_name", "save_pdf")
	usage := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/api_usage/"+usageID, admin, nil, http.StatusOK)
	usageOwner, _ := accessGroupsE2EFindString(usage, "user_account_id")
	state, _ := accessGroupsE2EFindString(usage, "state")
	if usageOwner != ownerID || state != "completed" {
		t.Fatalf("incorrect conversion usage owner or state: %#v", usage)
	}
	// Subject loading happens before temporary action privileges are added.
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, saveURL, other, savedPayload, http.StatusBadRequest)
	// Revocation is enforced on the next invocation, including a warm subject.
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, rowURL, admin,
		accessGroupsE2ERecordPayload("conversion_document", documentID, map[string]interface{}{"permission": int64(auth.UserPeek | auth.UserRead)}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, saveURL, owner, savedPayload, http.StatusForbidden)
	row = accessGroupsE2ERequestJSON(t, client, http.MethodGet, rowURL, owner, nil, http.StatusOK)
	unchanged, _ := accessGroupsE2EFindString(row, "output_base64")
	if unchanged != stored {
		t.Fatal("denied invocation changed persisted output")
	}

	// GraphQL must reach the same named action and performer.
	graph := transportE2EPostJSON(t, client, base+"/graphql", owner, map[string]interface{}{
		"query": fmt.Sprintf(`mutation { executeConvertDocumentOnConversionDocument(source_format: "txt", target_format: "pdf", file: %q) { ResponseType } }`, `[{"name":"source.txt","contents":"`+encoded+`"}]`),
	})
	graphMap, valid := graph.(map[string]interface{})
	responseType, _ := accessGroupsE2EFindString(graph, "ResponseType")
	if !valid || graphMap["errors"] != nil || responseType != "client.file.download" {
		t.Fatalf("GraphQL conversion: %#v", graph)
	}

	// An invalid conversion must stop before the persisted PATCH outcome.
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, rowURL, admin,
		accessGroupsE2ERecordPayload("conversion_document", documentID, map[string]interface{}{"source_format": "docx", "permission": int64(auth.UserCRUD | auth.UserExecute)}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, client, http.MethodPost, saveURL, owner, savedPayload, http.StatusBadRequest)
	row = accessGroupsE2ERequestJSON(t, client, http.MethodGet, rowURL, owner, nil, http.StatusOK)
	unchanged, _ = accessGroupsE2EFindString(row, "output_base64")
	if unchanged != stored {
		t.Fatal("failed conversion changed persisted output")
	}

	// Persist a normal task and restart to load it through Daptin's scheduler.
	scheduledID := accessGroupsE2ECreateRecord(t, client, base, owner, "conversion_document", map[string]interface{}{
		"title": "Scheduled private source", "source_format": "txt", "source_base64": encoded,
	})
	taskAttrs, err := json.Marshal(map[string]interface{}{"conversion_document_id": scheduledID})
	if err != nil {
		t.Fatal(err)
	}
	taskID := accessGroupsE2ECreateRecord(t, client, base, admin, "task", map[string]interface{}{
		"name": "convert-private-document", "job_type": "action", "entity_name": "conversion_document", "action_name": "save_pdf",
		"schedule": "@every 1s", "active": true, "as_user_id": ownerID, "attributes": string(taskAttrs),
	})
	fixture.Process.stopProcess()
	restarted := startDaptinE2E(t, options)
	client, base = restarted.Client, restarted.URL
	deadline := time.Now().Add(15 * time.Second)
	var scheduledOutput string
	for time.Now().Before(deadline) {
		row := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/conversion_document/"+scheduledID, owner, nil, http.StatusOK)
		scheduledOutput, _ = accessGroupsE2EFindString(row, "output_base64")
		if scheduledOutput != "" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if scheduledOutput == "" {
		t.Fatal("scheduled conversion did not persist output")
	}
	assertDocumentE2EPDF(t, map[string]interface{}{"content": scheduledOutput})
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/task/"+taskID, admin,
		accessGroupsE2ERecordPayload("task", taskID, map[string]interface{}{"active": false}), http.StatusOK)
	usageRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/api_usage?page[size]=1000", admin, nil, http.StatusOK))
	foundScheduledUsage := false
	for _, item := range usageRows {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["entity_type"] == "conversion_document" && attrs["action_name"] == "save_pdf" && attrs["endpoint"] == "/action/conversion_document" && attrs["state"] == "completed" {
			if attrs["user_account_id"] != ownerID {
				t.Fatalf("scheduled usage belongs to the wrong account: %#v", attrs)
			}
			foundScheduledUsage = true
		}
	}
	if !foundScheduledUsage {
		t.Fatalf("scheduled conversion usage missing: %#v", usageRows)
	}
}

func assertDocumentE2EPDF(t *testing.T, response interface{}) []byte {
	t.Helper()
	encoded, ok := accessGroupsE2EFindString(response, "content")
	if !ok {
		t.Fatalf("missing download content: %#v", response)
	}
	pdf, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("invalid PDF download: %v", err)
	}
	return pdf
}
