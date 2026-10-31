package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/google/uuid"
)

// The built-in read action must not inherit the action executor's temporary
// administrator membership, even when the message itself remains readable.
func TestMailBrowserReadRevocationRealE2E(t *testing.T) {
	fixture := startDaptinE2E(t, transportE2EDaptinOptions{})
	admin := fixture.SignupAdmin(t)
	alice := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "mail-alice")
	bob := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "mail-bob")
	aliceRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "mail-alice@test.local")
	private := int64(auth.UserCRUD | auth.UserExecute)
	seed := func(table string, values map[string]interface{}) string {
		ref := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, table, values)
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/"+table+"/"+ref, admin,
			accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"user_account_id": aliceRef, "permission": private}), http.StatusOK)
		return ref
	}
	serverRef := seed("mail_server", map[string]interface{}{"hostname": "browser.test", "is_enabled": false, "listen_interface": "127.0.0.1:0", "max_size": 10000, "max_clients": 1, "xclient_on": false, "always_on_tls": false, "authentication_required": true})
	accountRef := seed("mail_account", map[string]interface{}{"username": "mail-alice@browser.test", "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": serverRef})
	boxRef := seed("mail_box", map[string]interface{}{"name": "INBOX", "subscribed": true, "uidvalidity": 1, "nextuid": 43, "attributes": "", "flags": "", "permanent_flags": "\\Seen", "mail_account_id": accountRef})
	raw := "From: sender@example.test\r\nContent-Type: multipart/mixed; boundary=p\r\n\r\n--p\r\nContent-Type: text/plain\r\n\r\nPrivate body\r\n--p\r\nContent-Type: text/plain\r\nContent-Disposition: attachment; filename=proof.txt\r\nContent-Transfer-Encoding: base64\r\n\r\ncHJvb2Y=\r\n--p--\r\n"
	messageRef := seed("mail", map[string]interface{}{
		"mail_box_id": boxRef, "mail": base64.StdEncoding.EncodeToString([]byte(raw)), "message_id": "<browser@test>", "mail_id": "browser",
		"from_address": "sender@example.test", "internal_date": time.Now().UTC().Format(time.RFC3339), "to_address": "mail-alice@browser.test", "reply_to_address": "sender@example.test", "sender_address": "sender@example.test",
		"subject": "Private fixture", "body": "Private body", "spam_score": 0, "hash": "browser", "content_type": "multipart/mixed", "recipient": "mail-alice@browser.test", "has_attachment": true,
		"ip_addr": "127.0.0.1", "return_path": "sender@example.test", "is_tls": false, "seen": false, "recent": true, "deleted": false, "uid": 42, "spam": false, "size": len(raw), "flags": "",
	})
	payload := map[string]interface{}{"attributes": map[string]interface{}{"message_ref": messageRef}}
	url := fixture.URL + "/action/mail/unpack"
	response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, url, alice, payload, http.StatusOK)
	attachment := ""
	attributes := response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	for _, value := range attributes["parts"].([]interface{}) {
		part := value.(map[string]interface{})
		if part["filename"] == "proof.txt" {
			attachment = part["content"].([]interface{})[0].(map[string]interface{})["contents"].(string)
		}
	}
	if attachment != "cHJvb2Y=" {
		t.Fatalf("authorized MIME attachment missing: %#v", response)
	}
	if ref, _ := accessGroupsE2EFindString(response, "reference_id"); ref != messageRef {
		t.Fatalf("MIME reference must be canonical resource, not UID: %#v", response)
	}
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, url, bob, payload, http.StatusForbidden)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail_account/"+accountRef, admin,
		accessGroupsE2ERecordPayload("mail_account", accountRef, map[string]interface{}{"permission": 0}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, url, alice, payload, http.StatusForbidden)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_box/status", alice,
		map[string]interface{}{"attributes": map[string]interface{}{"mail_account_id": accountRef}}, http.StatusForbidden)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail_account/"+accountRef, admin,
		accessGroupsE2ERecordPayload("mail_account", accountRef, map[string]interface{}{"permission": private}), http.StatusOK)
	draft := map[string]interface{}{
		"account_ref": accountRef, "compose_key": uuid.NewString(), "write_key": uuid.NewString(),
		"content": map[string]interface{}{"subject": "Durable draft", "text": "First text", "html": "<p>First text</p>",
			"to": []map[string]interface{}{{"address": "recipient@example.test"}}, "bcc": []map[string]interface{}{{"address": "hidden@example.test"}},
			"in_reply_to": "<original@test>", "references": []string{"<root@test>", "<original@test>"},
			"parts": []map[string]interface{}{{"filename": "proof.txt", "media_type": "text/plain", "base64": "cHJvb2Y="}}},
	}
	draftURL := fixture.URL + "/action/mail_account/draft_save"
	draftPayload := func() map[string]interface{} {
		return map[string]interface{}{"attributes": map[string]interface{}{"draft": draft}}
	}
	created := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	draftRef, _ := accessGroupsE2EFindString(created, "reference_id")
	revision, _ := accessGroupsE2EFindString(created, "revision")
	if draftRef == "" || revision == "" {
		t.Fatalf("draft save omitted identity/revision: %#v", created)
	}
	// Lost-response retry returns the same canonical result without another UID.
	replayed := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	if ref, _ := accessGroupsE2EFindString(replayed, "reference_id"); ref != draftRef {
		t.Fatal("draft create replay duplicated the canonical message")
	}
	draft["draft_ref"] = draftRef
	draft["expected_revision"] = revision
	draft["write_key"] = uuid.NewString()
	draft["content"].(map[string]interface{})["text"] = "Second text"
	updated := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	nextRevision, _ := accessGroupsE2EFindString(updated, "revision")
	if nextRevision == revision {
		t.Fatal("draft update did not change MIME")
	}
	if ref, _ := accessGroupsE2EFindString(updated, "reference_id"); ref != draftRef {
		t.Fatal("draft update changed canonical identity")
	}
	draft["write_key"] = uuid.NewString() // stale expected revision remains rejected
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusConflict)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, draftURL, bob, draftPayload(), http.StatusForbidden)
	status := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_status", alice, draftPayload(), http.StatusOK)
	if recovered, _ := accessGroupsE2EFindString(status, "reference_id"); recovered != draftRef {
		t.Fatal("draft reconciliation lost identity")
	}
	draft["expected_revision"] = nextRevision
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_delete", bob, draftPayload(), http.StatusForbidden)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_delete", alice, draftPayload(), http.StatusOK)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_delete", alice, draftPayload(), http.StatusOK)
	// Two browser saves with the same revision may not both replace the draft.
	draft["compose_key"] = uuid.NewString()
	draft["write_key"] = uuid.NewString()
	draft["draft_ref"] = ""
	draft["expected_revision"] = ""
	concurrentDraft := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	concurrentRef, _ := accessGroupsE2EFindString(concurrentDraft, "reference_id")
	concurrentRevision, _ := accessGroupsE2EFindString(concurrentDraft, "revision")
	start := make(chan struct{})
	statuses := make(chan int, 2)
	for _, text := range []string{"Writer one", "Writer two"} {
		input := map[string]interface{}{"account_ref": accountRef, "compose_key": draft["compose_key"], "draft_ref": concurrentRef, "write_key": uuid.NewString(), "expected_revision": concurrentRevision, "content": map[string]interface{}{"subject": "Concurrent", "text": text}}
		payload, _ := json.Marshal(map[string]interface{}{"attributes": map[string]interface{}{"draft": input}})
		go func() {
			<-start
			request, _ := http.NewRequest(http.MethodPost, draftURL, bytes.NewReader(payload))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Authorization", "Bearer "+alice)
			response, err := fixture.Client.Do(request)
			if err != nil {
				statuses <- 0
				return
			}
			defer response.Body.Close()
			statuses <- response.StatusCode
		}()
	}
	close(start)
	first, second := <-statuses, <-statuses
	if !((first == http.StatusOK && second == http.StatusConflict) || (second == http.StatusOK && first == http.StatusConflict)) {
		t.Fatalf("concurrent saves must have one winner and one conflict, got %d/%d", first, second)
	}
	current := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_status", alice, draftPayload(), http.StatusOK)
	currentRevision, _ := accessGroupsE2EFindString(current, "revision")
	draft["expected_revision"] = currentRevision
	draft["draft_ref"] = concurrentRef
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_delete", alice, draftPayload(), http.StatusOK)

	// A native-shaped fixture has no browser key. Adoption preserves its current
	// mailbox UID and native flags; full network IMAP acceptance is separate.
	nativeBox := seed("mail_box", map[string]interface{}{"name": "NativeDrafts", "subscribed": true, "uidvalidity": 7, "nextuid": 10, "attributes": "", "flags": "", "permanent_flags": "\\*", "mail_account_id": accountRef})
	nativeRaw := "From: mail-alice@browser.test\r\nTo: recipient@example.test\r\nBcc: hidden@example.test\r\nSubject: Native draft\r\n\r\nNative text"
	nativeRef := seed("mail", map[string]interface{}{
		"mail_box_id": nativeBox, "mail": base64.StdEncoding.EncodeToString([]byte(nativeRaw)), "message_id": "<native@test>", "mail_id": "native-draft", "from_address": "mail-alice@browser.test", "sender_address": "mail-alice@browser.test", "reply_to_address": "mail-alice@browser.test", "to_address": "recipient@example.test", "subject": "Native draft", "body": "Native text",
		"internal_date": time.Now().UTC().Format(time.RFC3339), "spam_score": 0, "hash": "native-draft", "content_type": "text/plain", "recipient": "mail-alice@browser.test", "has_attachment": false, "ip_addr": "127.0.0.1", "return_path": "mail-alice@browser.test", "is_tls": false, "seen": false, "recent": true, "deleted": false, "uid": 9, "spam": false, "size": len(nativeRaw), "flags": "\\Draft,\\Recent,custom",
	})
	nativeSource := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/source", alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": nativeRef}}, http.StatusOK)
	nativeRevision, _ := accessGroupsE2EFindString(nativeSource, "revision")
	draft["compose_key"] = uuid.NewString()
	draft["write_key"] = uuid.NewString()
	draft["draft_ref"] = nativeRef
	draft["expected_revision"] = nativeRevision
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, draftURL, bob, draftPayload(), http.StatusForbidden)
	adopted := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	if ref, _ := accessGroupsE2EFindString(adopted, "reference_id"); ref != nativeRef {
		t.Fatal("native adoption duplicated canonical identity")
	}
	nativeRow := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail/"+nativeRef, admin, nil, http.StatusOK).(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if nativeRow["mail_box_id"] != nativeBox || nativeRow["uid"] != float64(9) || (nativeRow["recent"] != true && nativeRow["recent"] != float64(1)) || !strings.Contains(nativeRow["flags"].(string), "custom") {
		t.Fatalf("native adoption lost mailbox/UID/flags: box=%v uid=%v recent=%v flags=%v", nativeRow["mail_box_id"], nativeRow["uid"], nativeRow["recent"], nativeRow["flags"])
	}
	adoptedRevision, _ := accessGroupsE2EFindString(adopted, "revision")
	draft["expected_revision"] = adoptedRevision
	draft["compose_key"] = uuid.NewString()
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusForbidden)

	// A resource/native MIME change retains browser metadata but must invalidate
	// both same-write replay and lost-response confirmation.
	draft["compose_key"] = uuid.NewString()
	draft["write_key"] = uuid.NewString()
	draft["draft_ref"] = ""
	draft["expected_revision"] = ""
	guarded := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	guardedRef, _ := accessGroupsE2EFindString(guarded, "reference_id")
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail/"+guardedRef, admin,
		accessGroupsE2ERecordPayload("mail", guardedRef, map[string]interface{}{"mail": base64.StdEncoding.EncodeToString([]byte("From: mail-alice@browser.test\r\nTo: recipient@example.test\r\nSubject: External edit\r\n\r\nChanged outside browser"))}), http.StatusOK)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusConflict)
	changed := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_status", alice, draftPayload(), http.StatusOK)
	changedRevision, _ := accessGroupsE2EFindString(changed, "revision")
	writeRevision, _ := accessGroupsE2EFindString(changed, "write_revision")
	if changedRevision == writeRevision || writeRevision == "" {
		t.Fatalf("external MIME edit not attributable: %#v", changed)
	}
	draft["draft_ref"] = guardedRef
	draft["expected_revision"] = changedRevision
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_delete", alice, draftPayload(), http.StatusOK)

	// Queue-only submission: disabled SMTP, no delivery performer is invoked.
	certificateRef := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", map[string]interface{}{"hostname": "browser.test", "issuer": "self"})
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/certificate/generate_self_certificate", admin,
		map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificateRef}}, http.StatusOK)
	draft["compose_key"] = uuid.NewString()
	draft["write_key"] = uuid.NewString()
	draft["draft_ref"] = ""
	draft["expected_revision"] = ""
	sendDraft := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusOK)
	sendRef, _ := accessGroupsE2EFindString(sendDraft, "reference_id")
	sendRevision, _ := accessGroupsE2EFindString(sendDraft, "revision")
	submission := map[string]interface{}{"account_ref": accountRef, "compose_key": draft["compose_key"], "submission_key": uuid.NewString(), "expected_revision": sendRevision}
	submitPayload := func() map[string]interface{} {
		return map[string]interface{}{"attributes": map[string]interface{}{"submission": submission}}
	}
	submitURL := fixture.URL + "/action/mail_account/submit"
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, submitURL, bob, submitPayload(), http.StatusForbidden)
	submission["expected_revision"] = strings.Repeat("0", 64)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, submitURL, alice, submitPayload(), http.StatusConflict)
	submission["expected_revision"] = sendRevision
	queued := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, submitURL, alice, submitPayload(), http.StatusOK)
	if ref := queued.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})["reference_id"]; ref != sendRef {
		t.Fatalf("submission changed canonical identity: %#v", queued)
	}
	// Replaying a lost response returns the same submission without new records.
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, submitURL, alice, submitPayload(), http.StatusOK)
	outbox := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/outbox?page%5Bsize%5D=100", admin, nil, http.StatusOK)
	outboxRows := accessGroupsE2EDataArray(t, outbox)
	if len(outboxRows) != 2 {
		t.Fatalf("want To and Bcc once each, got %#v", outbox)
	}
	for _, item := range outboxRows {
		row := item.(map[string]interface{})
		attributes := row["attributes"].(map[string]interface{})
		if attributes["permission"] != float64(auth.UserPeek|auth.UserRead|auth.UserRefer) {
			t.Fatalf("outbox sender must have read/reference rights only: %v", attributes["permission"])
		}
		if attributes["user_account_id"] != aliceRef || attributes["sent_mail_id"] != sendRef {
			t.Fatalf("outbox lost sender ownership/Sent relation: %#v", row)
		}
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/outbox/"+row["id"].(string), alice,
			accessGroupsE2ERecordPayload("outbox", row["id"].(string), map[string]interface{}{"from_address": "spoof@example.test"}), http.StatusForbidden)

	}
	source := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/source", alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": sendRef}}, http.StatusOK)
	encoded, _ := accessGroupsE2EFindString(source, "raw_base64")
	wire, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), "Bcc:") || !strings.Contains(string(wire), "DKIM-Signature:") || !strings.Contains(string(wire), "In-Reply-To: <original@test>") {
		t.Fatalf("invalid signed outgoing headers: %s", wire)
	}
	readSent := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, url, alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": sendRef}}, http.StatusOK)
	if subject, _ := accessGroupsE2EFindString(readSent, "subject"); subject != "Durable draft" {
		t.Fatalf("sent MIME context lost: %#v", readSent)
	}
	statusURL := fixture.URL + "/action/mail_account/submission_status"
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, statusURL, bob, submitPayload(), http.StatusForbidden)
	reconciled := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, statusURL, alice, submitPayload(), http.StatusOK)
	if key, _ := accessGroupsE2EFindString(reconciled, "submission_key"); key != submission["submission_key"] {
		t.Fatalf("submission status lost attribution: %#v", reconciled)
	}
	// State comes from the existing outbox, never a client-invented delivered flag.
	for i, item := range outboxRows {
		row := item.(map[string]interface{})
		ref := row["id"].(string)
		changes := map[string]interface{}{"sent": true}
		if i == 1 {
			changes = map[string]interface{}{"sent": false, "retry_count": 5, "last_error": "SMTP delivery uncertain: local fixture"}
		}
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/outbox/"+ref, admin, accessGroupsE2ERecordPayload("outbox", ref, changes), http.StatusOK)
	}
	states := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, statusURL, alice, submitPayload(), http.StatusOK)
	attrs := states.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	seenStates := map[string]bool{}
	for _, recipient := range attrs["recipients"].([]interface{}) {
		seenStates[recipient.(map[string]interface{})["state"].(string)] = true
	}
	if !seenStates["smtp_accepted"] || !seenStates["uncertain"] {
		t.Fatalf("wrong delivery classification: %#v", states)
	}
	submission["submission_key"] = uuid.NewString()
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, submitURL, alice, submitPayload(), http.StatusConflict)
	draft["draft_ref"] = sendRef
	draft["expected_revision"] = sendRevision
	draft["write_key"] = uuid.NewString()
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, draftURL, alice, draftPayload(), http.StatusConflict)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/draft_delete", alice, draftPayload(), http.StatusConflict)

	t.Run("canonical message and folder operations", func(t *testing.T) {
		call := func(family, operation, token string, input map[string]interface{}, status int) map[string]interface{} {
			response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost,
				fixture.URL+"/action/mail_account/"+family+"_"+operation, token,
				map[string]interface{}{"attributes": map[string]interface{}{family: input}}, status)
			if status != http.StatusOK {
				return nil
			}
			return response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
		}
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail/"+messageRef, admin,
			accessGroupsE2ERecordPayload("mail", messageRef, map[string]interface{}{"flags": "Project,\\Recent"}), http.StatusOK)
		message := map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef}
		state := call("message", "state", alice, message, http.StatusOK)
		precondition := func() {
			message["expected_box_ref"] = state["mail_box_id"]
			message["expected_version"] = state["version"]
		}
		precondition()
		message["seen"], message["flagged"] = true, true
		call("message", "flags", bob, message, http.StatusForbidden)
		state = call("message", "flags", alice, message, http.StatusOK)
		if state["seen"] != true || state["flagged"] != true {
			t.Fatalf("flags not synchronized: %#v", state)
		}
		keyword := false
		for _, flag := range state["flags"].([]interface{}) {
			if flag == "Project" {
				keyword = true
			}
		}
		if !keyword {
			t.Fatal("browser flags removed a native keyword")
		}
		call("message", "flags", alice, message, http.StatusConflict)
		precondition()
		call("message", "delete", alice, message, http.StatusConflict)
		folder := map[string]interface{}{"account_ref": accountRef, "name": "Projects/2026"}
		child := call("folder", "create", alice, folder, http.StatusOK)
		call("folder", "create", alice, folder, http.StatusConflict)
		folder["name"] = "../invalid"
		call("folder", "create", alice, folder, http.StatusUnprocessableEntity)
		folder["name"] = "Projects"
		call("folder", "create", alice, folder, http.StatusConflict)
		otherAccount := seed("mail_account", map[string]interface{}{"username": "other-alice@browser.test", "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": serverRef})
		otherFolder := seed("mail_box", map[string]interface{}{"name": "INBOX", "subscribed": true, "uidvalidity": 1, "nextuid": 1, "attributes": "", "flags": "", "permanent_flags": "\\Seen", "mail_account_id": otherAccount})
		message["destination_ref"] = otherFolder
		call("message", "move", alice, message, http.StatusForbidden)
		message["destination_ref"] = child["reference_id"]
		state = call("message", "move", alice, message, http.StatusOK)
		if state["reference_id"] != messageRef || state["mail_box_id"] != child["reference_id"] || state["uid"] != float64(1) {
			t.Fatalf("move must preserve reference and allocate destination UID: %#v", state)
		}
		precondition()
		noop := call("message", "move", alice, message, http.StatusOK)
		if noop["uid"] != state["uid"] || noop["version"] != state["version"] {
			t.Fatal("same-folder move changed state")
		}
		sourceAfter := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/source", alice,
			map[string]interface{}{"attributes": map[string]interface{}{"message_ref": messageRef}}, http.StatusOK)
		rawAfter, _ := accessGroupsE2EFindString(sourceAfter, "raw_base64")
		if rawAfter != base64.StdEncoding.EncodeToString([]byte(raw)) {
			t.Fatal("flags/move changed authoritative MIME")
		}
		folder["folder_ref"], folder["expected_version"] = child["reference_id"], child["version"]
		call("folder", "delete", alice, folder, http.StatusConflict)
		folder["name"] = "Work/2026"
		renamed := call("folder", "rename", alice, folder, http.StatusOK)
		if renamed["reference_id"] != child["reference_id"] || renamed["uidvalidity"] != child["uidvalidity"] {
			t.Fatal("rename reset mailbox identity")
		}
		call("folder", "rename", alice, folder, http.StatusConflict)
		// A parent rename preserves descendant identities and validity.
		parentRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail_box", "name", "Work")
		parent := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail_box/"+parentRef, alice, nil, http.StatusOK).(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
		parentInput := map[string]interface{}{"account_ref": accountRef, "folder_ref": parentRef, "expected_version": parent["version"], "name": "WorkRenamed"}
		parentRenamed := call("folder", "rename", alice, parentInput, http.StatusOK)
		childAfter := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail_box/"+child["reference_id"].(string), alice, nil, http.StatusOK).(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
		if childAfter["name"] != "WorkRenamed/2026" || childAfter["uidvalidity"] != child["uidvalidity"] {
			t.Fatalf("parent rename lost descendant state: %#v", childAfter)
		}
		folder["expected_version"] = childAfter["version"]
		parentInput["expected_version"] = parentRenamed["version"]
		call("folder", "delete", alice, parentInput, http.StatusOK)
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail_box/"+child["reference_id"].(string), alice, nil, http.StatusOK)
		state = call("message", "archive", alice, message, http.StatusOK)
		precondition()
		state = call("message", "trash", alice, message, http.StatusOK)
		precondition()
		deleted := call("message", "delete", alice, message, http.StatusOK)
		if deleted["exists"] != false {
			t.Fatal("permanent deletion did not remove message")
		}
		missing := call("message", "state", alice, message, http.StatusOK)
		if missing["exists"] != false {
			t.Fatal("deleted message still exists")
		}
		call("folder", "delete", alice, folder, http.StatusOK)
		delete(folder, "folder_ref")
		delete(folder, "expected_version")
		recreated := call("folder", "create", alice, folder, http.StatusOK)
		if recreated["uidvalidity"] == child["uidvalidity"] {
			t.Fatal("recreated mailbox reused UIDVALIDITY")
		}
		// A Sent copy remains the attribution authority even if moved to Trash.
		message = map[string]interface{}{"account_ref": accountRef, "message_ref": sendRef}
		state = call("message", "state", alice, message, http.StatusOK)
		precondition()
		state = call("message", "trash", alice, message, http.StatusOK)
		precondition()
		call("message", "delete", alice, message, http.StatusConflict)
	})

}
