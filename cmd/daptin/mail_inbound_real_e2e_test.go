package main

import (
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

// SMTP delivery must create private, operable canonical mailbox records using
// the same contracts as the browser. No mail or mailbox row is seeded here.
func TestMailInboundSMTPRealE2E(t *testing.T) {
	requireRealE2E(t)
	t.Setenv("DAPTIN_DISABLE_SMTP", "false")
	address := fmt.Sprintf("127.0.0.1:%d", freeTransportE2EPort(t, make(map[int]bool)))
	options := mailProtocolDatabase(t)
	fixture := startDaptinE2E(t, options)
	admin := fixture.SignupAdmin(t)
	alice := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "inbound-alice")
	bob := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "inbound-bob")
	server := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": true, "listen_interface": address, "max_size": 30000000,
		"max_clients": 4, "xclient_on": false, "always_on_tls": false, "authentication_required": false,
	})
	account := func(name string) string {
		owner := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", name+"@test.local")
		ref := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_account", map[string]interface{}{
			"username": name + "@localhost", "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": server,
		})
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail_account/"+ref, admin,
			accessGroupsE2ERecordPayload("mail_account", ref, map[string]interface{}{"user_account_id": owner, "permission": int64(auth.UserCRUD | auth.UserExecute)}), http.StatusOK)
		return ref
	}
	aliceAccount, bobAccount := account("inbound-alice"), account("inbound-bob")
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)

	conn, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	client, err := smtp.NewClient(conn, "localhost")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if err := client.Mail("sender@example.test"); err != nil {
		t.Fatal(err)
	}
	for _, recipient := range []string{"inbound-alice@localhost", "inbound-bob@localhost"} {
		if err := client.Rcpt(recipient); err != nil {
			t.Fatal(err)
		}
	}
	raw := strings.Join([]string{
		"From: Sender <sender@example.test>", "To: inbound-alice@localhost", // Bob receives via the envelope only.
		"Reply-To: reply@example.test", "Message-ID: <inbound-fixture@example.test>", "Subject: Inbound SMTP fixture",
		"MIME-Version: 1.0", "Content-Type: multipart/mixed; boundary=fixture", "",
		"--fixture", "Content-Type: multipart/alternative; boundary=alt", "",
		"--alt", "Content-Type: text/plain; charset=utf-8", "", "Delivered body",
		"--alt", "Content-Type: text/html; charset=utf-8", "Content-Transfer-Encoding: x-unknown-fixture",
		"", "<p>Opaque branch</p>", "--alt--",
		"--fixture", "Content-Type: application/octet-stream", "Content-Disposition: attachment; filename=damaged.bin",
		"Content-Transfer-Encoding: base64", "", "cHJlZml4!!!!",
		"--fixture", "Content-Type: application/octet-stream", "Content-Disposition: attachment; filename=proof.bin",
		"Content-Transfer-Encoding: base64", "", "AP9wcm9vZg==", "--fixture--", "",
	}, "\r\n")
	data, err := client.Data()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = data.Write([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if err = data.Close(); err != nil {
		t.Fatal(err)
	}
	if err = client.Quit(); err != nil {
		t.Fatal(err)
	}

	action := func(token, route string, input map[string]interface{}, status int) interface{} {
		t.Helper()
		return accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/"+route,
			token, map[string]interface{}{"attributes": input}, status)
	}
	checkInbox := func(token, accountRef string) (string, string) {
		t.Helper()
		response := action(token, "mail_box/status", map[string]interface{}{"mail_account_id": accountRef}, http.StatusOK)
		payload := response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
		rows := payload["data"].([]interface{})
		if len(rows) != 1 {
			t.Fatalf("expected one automatically created inbox: %#v", rows)
		}
		row := rows[0].(map[string]interface{})
		box, _ := row["mail_box_id"].(string)
		if box == "" || row["name"] != "INBOX" || row["messages"] != float64(1) ||
			row["unseen"] != float64(1) || row["uidnext"] != float64(2) {
			t.Fatalf("unexpected inbound mailbox status: %#v", row)
		}
		result := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail?fields=uid,has_attachment,subject,recent,flags", token, nil, http.StatusOK)
		messages := accessGroupsE2EDataArray(t, result)
		if len(messages) != 1 {
			t.Fatalf("expected one private message in the metadata list, got %d", len(messages))
		}
		record := messages[0].(map[string]interface{})
		ref, _ := record["id"].(string)
		attrs := record["attributes"].(map[string]interface{})
		attachmentFlag := attrs["has_attachment"]
		if ref == "" || attrs["subject"] != "Inbound SMTP fixture" || attrs["uid"] != float64(1) ||
			(attachmentFlag != true && attachmentFlag != float64(1)) {
			t.Fatalf("unexpected inbound metadata: %#v", attrs)
		}
		storedFlags, _ := attrs["flags"].(string)
		if (attrs["recent"] != true && attrs["recent"] != float64(1)) || strings.Contains(strings.ToLower(storedFlags), "\\recent") {
			t.Fatalf("inbound SMTP did not store the arrival state only in recent: %v %q", attrs["recent"], storedFlags)
		}
		for _, field := range []string{"mail", "body"} {
			if _, present := attrs[field]; present {
				t.Fatalf("sparse metadata included %s", field)
			}
		}
		rawResult := action(token, "mail/source", map[string]interface{}{"message_ref": ref}, http.StatusOK)
		encoded, ok := accessGroupsE2EFindString(rawResult, "raw_base64")
		// The SMTP receiver normalizes DATA line endings before storage.
		if !ok || encoded != base64.StdEncoding.EncodeToString([]byte(strings.ReplaceAll(raw, "\r\n", "\n"))) {
			t.Fatal("canonical source differs from the SMTP receiver's normalized MIME")
		}
		body := action(token, "mail/unpack", map[string]interface{}{"message_ref": ref}, http.StatusOK)
		if text, _ := accessGroupsE2EFindString(body, "preferred_text_body"); !strings.Contains(text, "Delivered body") {
			t.Fatalf("decoded body missing: %q", text)
		}
		bodyAttrs := body.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
		if bodyAttrs["optional_html_body"] != "" {
			t.Fatal("unsupported HTML was presented as a decoded body")
		}
		opaqueFound, damagedFound := false, false
		attachment := ""
		for _, value := range bodyAttrs["parts"].([]interface{}) {
			part := value.(map[string]interface{})
			if part["presentation"] == "unselected" && part["media_type"] == "application/octet-stream" {
				opaqueFound = true
			}
			if part["filename"] == "proof.bin" {
				attachment, _ = accessGroupsE2EFindString(part, "contents")
			}
			if part["filename"] == "damaged.bin" {
				data, present := accessGroupsE2EFindString(part, "contents")
				damagedFound = present && data == "" && part["presentation"] == "damaged"
			}
		}
		if attachment != "AP9wcm9vZg==" {
			t.Fatalf("attachment bytes differ: %q", attachment)
		}
		if !opaqueFound {
			t.Fatal("unsupported alternative lost its stable unpack leaf")
		}
		if !damagedFound {
			t.Fatal("damaged attachment was not isolated from its readable siblings")
		}
		return box, ref
	}
	aliceBox, aliceMessage := checkInbox(alice, aliceAccount)
	bobBox, bobMessage := checkInbox(bob, bobAccount)
	if aliceBox == bobBox || aliceMessage == bobMessage {
		t.Fatal("different envelope recipients must have distinct canonical records")
	}
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/unpack", bob,
		map[string]interface{}{"attributes": map[string]interface{}{"message_ref": aliceMessage}}, http.StatusForbidden)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/source", alice,
		map[string]interface{}{"attributes": map[string]interface{}{"message_ref": bobMessage}}, http.StatusForbidden)
	flags := action(alice, "mail_account/message_flags", map[string]interface{}{"message": map[string]interface{}{
		"account_ref": aliceAccount, "message_ref": aliceMessage, "expected_box_ref": aliceBox, "expected_version": 1, "seen": true,
	}}, http.StatusOK)
	flagState := flags.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	if flagState["reference_id"] != aliceMessage || flagState["seen"] != true {
		t.Fatalf("flag change did not retain the canonical message and mark it read: %#v", flagState)
	}
	status := action(alice, "mail_box/status", map[string]interface{}{"mail_account_id": aliceAccount}, http.StatusOK)
	rows := status.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})["data"].([]interface{})
	if len(rows) != 1 || rows[0].(map[string]interface{})["unseen"] != float64(0) {
		t.Fatalf("owner's unread count did not change: %#v", rows)
	}
	checkInbox(bob, bobAccount)
}
