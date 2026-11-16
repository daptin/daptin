package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

const mailUnpackE2ESchema = `
Actions:
  - Name: unpack_e2e
    Label: Unpack mail E2E
    OnType: mail
    InstanceOptional: false
    RequestSubjectRelations:
      - mail
    Permission: 2097152
    OutFields:
      - Type: mail.unpack
        Method: EXECUTE
        Reference: unpacked
        SkipInResponse: true
        Attributes:
          mail: "~subject.mail"
      - Type: mail.unpack
        Method: ACTIONRESPONSE
        Attributes:
          preferred_text_body: "~unpacked.preferred_text_body"
          optional_html_body: "~unpacked.optional_html_body"
          parts: "~unpacked.parts"
`

const mailUnpackCloudStoreE2ESchema = mailUnpackE2ESchema + `
Tables:
  - TableName: mail
    Columns:
      - Name: mail
        ColumnName: mail
        DataType: blob
        ColumnType: gzip
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: localstore
          KeyName: mail-messages
`

func TestMailUnpackActionCompositionRealE2E(t *testing.T) {
	requireRealE2E(t)
	t.Run("database-backed mail", func(t *testing.T) {
		testMailUnpackActionCompositionRealE2E(t, mailUnpackE2ESchema, false)
	})
	t.Run("cloud-store-backed mail", func(t *testing.T) {
		testMailUnpackActionCompositionRealE2E(t, mailUnpackCloudStoreE2ESchema, true)
	})
}

func testMailUnpackActionCompositionRealE2E(t *testing.T, schema string, cloudStore bool) {
	t.Helper()
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: schema})
	baseURL, client := fixture.URL, fixture.Client
	adminToken := fixture.SignupAdmin(t)
	otherToken := accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "mail-unpack-other")

	mailWorldID := accessGroupsE2EFindResourceID(t, client, baseURL, adminToken, "world", "table_name", "mail")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, baseURL+"/api/world/"+mailWorldID, adminToken,
		accessGroupsE2ERecordPayload("world", mailWorldID, map[string]interface{}{
			"permission": int64(auth.AuthenticatedExecute),
		}), http.StatusOK)
	mailServerID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mail_server", map[string]interface{}{
		"hostname":                "mail-unpack-e2e.test",
		"is_enabled":              false,
		"listen_interface":        "127.0.0.1:0",
		"max_size":                10000,
		"max_clients":             1,
		"xclient_on":              false,
		"always_on_tls":           false,
		"authentication_required": true,
	})
	mailAccountID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mail_account", map[string]interface{}{
		"username":       "mail-unpack@mail-unpack-e2e.test",
		"password":       "testpass123",
		"password_md5":   "testpass123",
		"mail_server_id": mailServerID,
	})
	mailBoxID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mail_box", map[string]interface{}{
		"name":            "INBOX",
		"subscribed":      true,
		"uidvalidity":     1,
		"nextuid":         2,
		"attributes":      "",
		"flags":           "\\Recent",
		"permanent_flags": "\\Seen",
		"mail_account_id": mailAccountID,
	})

	raw := []byte("From: sender@example.com\r\nContent-Type: text/plain; charset=utf-8\r\n\r\ncomposed mail body")
	mailValue := interface{}(base64.StdEncoding.EncodeToString(raw))
	if cloudStore {
		mailValue = []interface{}{map[string]interface{}{
			"name": "message.eml",
			"file": "data:message/rfc822;base64," + base64.StdEncoding.EncodeToString(raw),
			"type": "message/rfc822",
		}}
	}
	mailID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mail", map[string]interface{}{
		"message_id":       "mail-unpack-e2e@test.local",
		"mail_id":          "mail-unpack-e2e",
		"from_address":     "sender@example.com",
		"internal_date":    time.Now().UTC().Format(time.RFC3339),
		"to_address":       "receiver@example.com",
		"reply_to_address": "sender@example.com",
		"sender_address":   "sender@example.com",
		"subject":          "Mail unpack E2E",
		"body":             "composed mail body",
		"mail":             mailValue,
		"spam_score":       0,
		"hash":             "mail-unpack-e2e",
		"content_type":     "text/plain; charset=utf-8",
		"recipient":        "receiver@example.com",
		"has_attachment":   false,
		"ip_addr":          "127.0.0.1",
		"return_path":      "sender@example.com",
		"is_tls":           false,
		"seen":             false,
		"recent":           true,
		"deleted":          false,
		"uid":              1,
		"spam":             false,
		"size":             len(raw),
		"flags":            "\\Recent",
		"mail_box_id":      mailBoxID,
		"permission":       int64(auth.UserRead | auth.UserExecute),
	})

	if cloudStore {
		assetURL := baseURL + "/asset/mail/" + mailID + "/mail"
		// Record creation queues the copy. Wait for the authorized asset route
		// to serve the wire bytes before exercising the action exactly once.
		waitForMailFixtureAsset(t, client, assetURL, adminToken, raw)
		request, err := http.NewRequest(http.MethodGet, assetURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+otherToken)
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusForbidden && response.StatusCode != http.StatusNotFound {
			t.Fatalf("second user received HTTP %d from the warm mail asset route", response.StatusCode)
		}
	}

	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/mail/unpack_e2e?mail_id="+mailID, adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	body, ok := accessGroupsE2EFindString(response, "preferred_text_body")
	if !ok || body != "composed mail body" {
		t.Fatalf("mail.unpack response body = %q, found=%v: %#v", body, ok, response)
	}

	accessGroupsE2EAssertStatus(t, client, http.MethodPost,
		baseURL+"/action/mail/unpack_e2e?mail_id="+mailID, otherToken,
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusBadRequest)
}

func waitForMailFixtureAsset(t *testing.T, client *http.Client, url, token string, expected []byte) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for attempts := 1; ; attempts++ {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		response, err := client.Do(request)
		if err != nil {
			t.Fatalf("probe queued mail asset: %v", err)
		}
		status := response.StatusCode
		data, err := io.ReadAll(io.LimitReader(response.Body, int64(len(expected))+1))
		response.Body.Close()
		if err != nil {
			t.Fatalf("read queued mail asset: %v", err)
		}
		if status == http.StatusOK {
			if !bytes.Equal(data, expected) {
				t.Fatalf("mail asset returned %d unexpected bytes, want %d exact bytes", len(data), len(expected))
			}
			t.Logf("cloud mail bytes readable after %d asset request(s)", attempts)
			return
		}
		if status != http.StatusNotFound {
			t.Fatalf("probe queued mail asset: unexpected HTTP %d", status)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("mail asset still missing after %d request(s): %v", attempts, ctx.Err())
		case <-ticker.C:
		}
	}
}
