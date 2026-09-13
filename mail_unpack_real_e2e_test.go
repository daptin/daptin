package main

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
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
	if os.Getenv("DAPTIN_REAL_E2E") != "1" {
		t.Skip("set DAPTIN_REAL_E2E=1 to run the mail.unpack action composition e2e")
	}
	t.Run("database-backed mail", func(t *testing.T) {
		testMailUnpackActionCompositionRealE2E(t, mailUnpackE2ESchema, false, true)
	})
	t.Run("cloud-store-backed mail", func(t *testing.T) {
		testMailUnpackActionCompositionRealE2E(t, mailUnpackCloudStoreE2ESchema, true, false)
	})
}

func testMailUnpackActionCompositionRealE2E(t *testing.T, schema string, cloudStore bool, verifyDenial bool) {
	t.Helper()
	usedPorts := make(map[int]bool, 2)
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	daptinProcess := startTransportE2EDaptin(t, port, httpsPort, baseURL, transportE2EDaptinOptions{schema: schema})
	defer daptinProcess.stopProcess()

	client := &http.Client{Timeout: 20 * time.Second}
	adminToken := accessGroupsE2ESignupSigninAdmin(t, client, baseURL)
	var otherToken string
	if verifyDenial {
		otherToken = accessGroupsE2ESignupSigninUser(t, client, baseURL, adminToken, "mail-unpack-other")
	}

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

	response := accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/mail/unpack_e2e?mail_id="+mailID, adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	body, ok := accessGroupsE2EFindString(response, "preferred_text_body")
	if !ok || body != "composed mail body" {
		t.Fatalf("mail.unpack response body = %q, found=%v: %#v", body, ok, response)
	}

	if verifyDenial {
		accessGroupsE2EAssertStatus(t, client, http.MethodPost,
			baseURL+"/action/mail/unpack_e2e?mail_id="+mailID, otherToken,
			map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusBadRequest)
	}
}
