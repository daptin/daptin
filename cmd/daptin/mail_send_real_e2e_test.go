package main

import (
	"net/http"
	"testing"
)

const guestMailSendE2ESchema = `
Actions:
  - Name: send_guest_mail
    Label: Send guest mail
    OnType: user_account
    InstanceOptional: true
    Permission: 32
    OutFields:
      - Type: mail.send
        Method: EXECUTE
        Attributes:
          from: no-reply@localhost
          to: recipient@example.test
          subject: Guest mail
          body: Sent by a guest action
`

func TestGuestMailSendUsesConfiguredSenderRealE2E(t *testing.T) {
	requireRealE2E(t)
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{schema: guestMailSendE2ESchema})
	baseURL, client := fixture.URL, fixture.Client
	adminToken := fixture.SignupAdmin(t)

	mailServerID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mail_server", map[string]interface{}{
		"hostname":                "localhost",
		"is_enabled":              false,
		"listen_interface":        "127.0.0.1:0",
		"max_size":                10000,
		"max_clients":             1,
		"xclient_on":              false,
		"always_on_tls":           false,
		"authentication_required": true,
	})
	mailAccountID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "mail_account", map[string]interface{}{
		"username":       "no-reply@localhost",
		"password":       "testpass123",
		"password_md5":   "testpass123",
		"mail_server_id": mailServerID,
	})
	certificateID := accessGroupsE2ECreateRecord(t, client, baseURL, adminToken, "certificate", map[string]interface{}{
		"hostname": "localhost",
		"issuer":   "self",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		baseURL+"/action/certificate/generate_self_certificate", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificateID}}, http.StatusOK)

	accessGroupsE2ERequestJSON(t, client, http.MethodPost, baseURL+"/action/user_account/send_guest_mail", "",
		map[string]interface{}{"attributes": map[string]interface{}{}}, http.StatusOK)
	accountResponse := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/mail_account/"+mailAccountID, adminToken, nil, http.StatusOK).(map[string]interface{})
	accountData := accountResponse["data"].(map[string]interface{})
	accountAttributes := accountData["attributes"].(map[string]interface{})
	mailOwner := accountAttributes["user_account_id"]
	if mailOwner == nil {
		t.Fatalf("sender mail account has no owner: %#v", accountResponse)
	}

	mailResponse := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)
	var foundSent bool
	for _, item := range accessGroupsE2EDataArray(t, mailResponse) {
		row, _ := item.(map[string]interface{})
		attributes, _ := row["attributes"].(map[string]interface{})
		if attributes["subject"] == "Guest mail" && attributes["user_account_id"] == mailOwner {
			foundSent = true
		}
	}
	if !foundSent {
		t.Fatalf("guest mail action did not create sender Sent copy: %#v", mailResponse)
	}
	outboxResponse := accessGroupsE2ERequestJSON(t, client, http.MethodGet, baseURL+"/api/outbox?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)
	outboxRows := accessGroupsE2EDataArray(t, outboxResponse)
	if len(outboxRows) != 1 {
		t.Fatalf("guest mail action enqueued %d outbox rows, want one: %#v", len(outboxRows), outboxResponse)
	}
	outboxRow := outboxRows[0].(map[string]interface{})
	outboxAttributes := outboxRow["attributes"].(map[string]interface{})
	if outboxAttributes["from_address"] != "no-reply@localhost" || outboxAttributes["to_address"] != "recipient@example.test" {
		t.Fatalf("guest mail action queued wrong sender or recipient: %#v", outboxAttributes)
	}
}
