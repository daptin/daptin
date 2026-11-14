package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/artpar/go-imap"
	imapclient "github.com/artpar/go-imap/client"
	"github.com/daptin/daptin/server/auth"
	"github.com/google/uuid"
)

// This test connects to the actual STARTTLS IMAP listener of an isolated Daptin
// executable. Resource/action HTTP remains the setup and browser authority.
func TestMailNativeRoundTripRealE2E(t *testing.T) {
	requireRealE2E(t)
	database := filepath.Join(t.TempDir(), "native-mail.db")
	options := transportE2EDaptinOptions{connectionString: database}
	fixture := startDaptinE2E(t, options)
	admin := fixture.SignupAdmin(t)
	alice := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "native-alice")
	aliceRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "native-alice@test.local")
	private := int64(auth.UserCRUD | auth.UserExecute)
	seed := func(table string, values map[string]interface{}) string {
		ref := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, table, values)
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/"+table+"/"+ref, admin,
			accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"user_account_id": aliceRef, "permission": private}), http.StatusOK)
		return ref
	}
	serverRef := seed("mail_server", map[string]interface{}{"hostname": "native.test", "is_enabled": false, "listen_interface": "127.0.0.1:0", "max_size": 30000000, "max_clients": 1, "xclient_on": false, "always_on_tls": false, "authentication_required": true})
	accountRef := seed("mail_account", map[string]interface{}{"username": "native-alice@native.test", "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": serverRef})
	cert := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", map[string]interface{}{"hostname": "imap.native.test", "issuer": "self"})
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/certificate/generate_self_certificate", admin,
		map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": cert}}, http.StatusOK)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	configure := func(key, value string) {
		request, err := http.NewRequest(http.MethodPost, fixture.URL+"/_config/backend/"+key, strings.NewReader(value))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+admin)
		response, err := fixture.Client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("configure %s: HTTP %d", key, response.StatusCode)
		}
	}
	configure("imap.listen_interface", address)
	configure("imap.hostname", "imap.native.test")
	configure("imap.enabled", "true")
	fixture.Process.stopProcess()
	fixture = startDaptinE2E(t, options)
	t.Cleanup(func() {
		if t.Failed() {
			output := fixture.Process.logs.String()
			if len(output) > 6000 {
				output = output[len(output)-6000:]
			}
			t.Log(output)
		}
	})
	// TLS skips certificate verification only for this generated, loopback-only
	// self-signed fixture. Production listener policy remains unchanged.
	native, err := imapclient.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, address)
	if err != nil {
		t.Fatalf("IMAP connect: %v", err)
	}
	native.Timeout = 15 * time.Second
	t.Cleanup(func() { _ = native.Terminate() })
	if err = native.StartTLS(&tls.Config{InsecureSkipVerify: true}); err != nil {
		t.Fatalf("STARTTLS: %v", err)
	}
	if err = native.Login("native-alice@native.test", "fixture-password"); err != nil {
		t.Fatalf("native login: %v", err)
	}
	list := func(wantError bool) []*imap.MailboxInfo {
		channel := make(chan *imap.MailboxInfo, 100)
		err := native.List("", "*", channel)
		var boxes []*imap.MailboxInfo
		for info := range channel {
			boxes = append(boxes, info)
		}
		if wantError {
			if err == nil {
				t.Fatal("revoked native LIST succeeded")
			}
			return boxes
		}
		if err != nil {
			t.Fatalf("native LIST: %v", err)
		}
		for _, box := range boxes {
			if box.Delimiter != "/" {
				t.Fatalf("native hierarchy delimiter = %q", box.Delimiter)
			}
		}
		return boxes
	}
	if len(list(false)) < 6 {
		t.Fatal("native discovery did not create canonical folders")
	}
	call := func(name, family string, input map[string]interface{}) map[string]interface{} {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/"+name, alice,
			map[string]interface{}{"attributes": map[string]interface{}{family: input}}, http.StatusOK)
		return response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	}
	draft := map[string]interface{}{"account_ref": accountRef, "compose_key": uuid.NewString(), "write_key": uuid.NewString(),
		"content": map[string]interface{}{"subject": "Browser round trip", "text": "مرحبا नमस्ते こんにちは", "html": "<p>مرحبا नमस्ते こんにちは</p><img src=\"cid:inline@native\">",
			"to": []map[string]interface{}{{"address": "recipient@example.test"}}, "bcc": []map[string]interface{}{{"address": "hidden@example.test"}},
			"in_reply_to": "<original@test>", "references": []string{"<root@test>", "<original@test>"},
			"parts": []map[string]interface{}{{"filename": "inline.png", "media_type": "image/png", "content_id": "inline@native", "base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aH2kAAAAASUVORK5CYII="}, {"filename": "résumé.txt", "media_type": "text/plain", "base64": "cHJvb2Y="}}}}
	saved := call("draft_save", "draft", draft)
	messageRef := saved["reference_id"].(string)
	uid := uint32(saved["uid"].(float64))
	source := func(ref string) []byte {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/source", alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": ref}}, http.StatusOK)
		encoded, _ := accessGroupsE2EFindString(response, "raw_base64")
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	raw := source(messageRef)
	if _, err = native.Select("Draft", false); err != nil {
		t.Fatalf("native SELECT: %v", err)
	}
	seq := new(imap.SeqSet)
	seq.AddNum(uid)
	fetch := func(wantError bool) *imap.Message {
		channel := make(chan *imap.Message, 10)
		section := &imap.BodySectionName{Peek: true}
		err := native.UidFetch(seq, []imap.FetchItem{imap.FetchUid, imap.FetchFlags, section.FetchItem()}, channel)
		var messages []*imap.Message
		for message := range channel {
			messages = append(messages, message)
		}
		if wantError {
			if err == nil {
				t.Fatal("revoked native FETCH succeeded")
			}
			if len(messages) != 0 {
				t.Fatal("revoked native FETCH released message data")
			}
			return nil
		}
		if err != nil || len(messages) != 1 {
			t.Fatalf("native FETCH: %v, %d messages", err, len(messages))
		}
		body, err := io.ReadAll(messages[0].GetBody(section))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, source(messageRef)) {
			t.Fatal("native/browser original MIME differs")
		}
		return messages[0]
	}
	if fetch(false).Uid != uid {
		t.Fatal("canonical browser record has different native UID")
	}
	if err = native.UidStore(seq, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.FlaggedFlag, "Project"}, nil); err != nil {
		t.Fatalf("native STORE: %v", err)
	}
	state := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef})
	if state["flagged"] != true {
		t.Fatal("native STORE not visible to browser")
	}
	call("message_flags", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef, "expected_box_ref": state["mail_box_id"], "expected_version": state["version"], "seen": false})
	flags := strings.Join(fetch(false).Flags, ",")
	if !strings.Contains(strings.ToLower(flags), "project") || !strings.Contains(flags, imap.FlaggedFlag) || strings.Contains(flags, imap.SeenFlag) {
		t.Fatalf("native/browser flags disagree: %s", flags)
	}
	if err = native.Create("Native/2026"); err != nil {
		t.Fatalf("native CREATE hierarchy: %v", err)
	}
	nativeRaw := bytes.Replace(raw, []byte("Browser round trip"), []byte("Native round trip"), 1)
	nativeDate := time.Date(2024, 2, 3, 4, 5, 6, 0, time.UTC)
	if err = native.Append("Native/2026", []string{imap.DraftFlag, imap.FlaggedFlag, "Project"}, nativeDate, bytes.NewReader(nativeRaw)); err != nil {
		t.Fatalf("native APPEND: %v", err)
	}
	nativeRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Native round trip")
	if !bytes.Equal(source(nativeRef), nativeRaw) {
		t.Fatal("native APPEND changed original MIME")
	}
	nativeState := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": nativeRef})
	parsed := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/unpack", alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": nativeRef}}, http.StatusOK)
	revision, _ := accessGroupsE2EFindString(parsed, "revision")
	draft["draft_ref"], draft["compose_key"], draft["write_key"], draft["expected_revision"] = nativeRef, uuid.NewString(), uuid.NewString(), revision
	draft["content"].(map[string]interface{})["subject"] = "Adopted native draft"
	adopted := call("draft_save", "draft", draft)
	adoptedState := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": nativeRef})
	if adopted["reference_id"] != nativeRef || adoptedState["uid"] != nativeState["uid"] || adoptedState["mail_box_id"] != nativeState["mail_box_id"] || adoptedState["flagged"] != true {
		t.Fatal("native draft adoption lost canonical identity/flags")
	}
	// Empty TEXT is a valid literal; it must not make FETCH omit the message.
	empty := []byte("From: native-alice@native.test\r\nTo: recipient@example.test\r\nSubject: Empty native body\r\n\r\n")
	if err = native.Append("Native/2026", nil, nativeDate, bytes.NewReader(empty)); err != nil {
		t.Fatal(err)
	}
	if _, err = native.Select("Native/2026", false); err != nil {
		t.Fatal(err)
	}
	emptySeq := new(imap.SeqSet)
	emptySeq.AddNum(2)
	emptySection := &imap.BodySectionName{Peek: true, BodyPartName: imap.BodyPartName{Specifier: imap.TextSpecifier}}
	emptyMessages := make(chan *imap.Message, 2)
	if err = native.UidFetch(emptySeq, []imap.FetchItem{imap.FetchUid, emptySection.FetchItem()}, emptyMessages); err != nil {
		t.Fatal(err)
	}
	var emptyCount int
	for value := range emptyMessages {
		literal := value.GetBody(emptySection)
		if literal == nil {
			t.Fatal("empty TEXT literal omitted")
		}
		content, err := io.ReadAll(literal)
		if err != nil || len(content) != 0 {
			t.Fatal("invalid empty TEXT literal")
		}
		emptyCount++
	}
	if emptyCount != 1 {
		t.Fatal("empty-body message omitted from native FETCH")
	}
	if _, err = native.Select("Draft", false); err != nil {
		t.Fatal(err)
	}
	// The upstream client blocks its literal writer on a rejected continuation,
	// so exercise the over-limit wire declaration directly with a deadline.
	bounded, err := net.DialTimeout("tcp", address, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = bounded.SetDeadline(time.Now().Add(5 * time.Second))
	wireReader := bufio.NewReader(bounded)
	if _, err = wireReader.ReadString('\n'); err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(bounded, "a STARTTLS\r\n"); err != nil {
		t.Fatal(err)
	}
	if line, err := wireReader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "a OK") {
		t.Fatalf("bounded STARTTLS: %q %v", line, err)
	}
	secure := tls.Client(bounded, &tls.Config{InsecureSkipVerify: true})
	if err = secure.Handshake(); err != nil {
		t.Fatal(err)
	}
	wireReader = bufio.NewReader(secure)
	if _, err = io.WriteString(secure, "b LOGIN native-alice@native.test fixture-password\r\n"); err != nil {
		t.Fatal(err)
	}
	if line, err := wireReader.ReadString('\n'); err != nil || !strings.HasPrefix(line, "b OK") {
		t.Fatalf("bounded LOGIN: %q %v", line, err)
	}
	if _, err = io.WriteString(secure, "c APPEND Draft {27262976}\r\n"); err != nil {
		t.Fatal(err)
	}
	response, err := wireReader.ReadString('\n')
	_ = secure.Close()
	if err != nil || !(strings.HasPrefix(response, "c BAD") || strings.HasPrefix(response, "c NO") || strings.HasPrefix(response, "* BAD literal exceeding maximum size")) {
		t.Fatalf("oversized APPEND not rejected before literal transfer: %q %v", response, err)
	}

	if err = native.Create("Copies"); err != nil {
		t.Fatal(err)
	}
	if err = native.UidCopy(seq, "Copies"); err != nil {
		t.Fatalf("native COPY: %v", err)
	}
	copyBox := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail_box", "name", "Copies")
	filter, _ := json.Marshal([]map[string]interface{}{{"column": "mail_box_id", "operator": "=", "value": copyBox}})
	copied := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail?page[size]=10&query="+url.QueryEscape(string(filter))+"&fields=reference_id,compose_key,submission_key", alice, nil, http.StatusOK)
	rows := accessGroupsE2EDataArray(t, copied)
	if len(rows) != 1 {
		t.Fatalf("native COPY created %d records", len(rows))
	}
	copyRow := rows[0].(map[string]interface{})
	copyRef := copyRow["id"].(string)
	copyAttrs := copyRow["attributes"].(map[string]interface{})
	if copyRef == messageRef || copyAttrs["compose_key"] != nil && copyAttrs["compose_key"] != "" || copyAttrs["submission_key"] != nil && copyAttrs["submission_key"] != "" {
		t.Fatal("COPY reused browser operation identity")
	}
	if !bytes.Equal(source(copyRef), raw) {
		t.Fatal("COPY changed MIME")
	}
	if err = native.Unsubscribe("Copies"); err != nil {
		t.Fatalf("native UNSUBSCRIBE: %v", err)
	}
	subscribed := make(chan *imap.MailboxInfo, 100)
	if err = native.Lsub("", "*", subscribed); err != nil {
		t.Fatal(err)
	}
	for info := range subscribed {
		if info.Name == "Copies" {
			t.Fatal("LSUB returned an unsubscribed folder")
		}
	}
	if err = native.Subscribe("Copies"); err != nil {
		t.Fatalf("native SUBSCRIBE: %v", err)
	}

	if err = native.UidStore(seq, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.DeletedFlag}, nil); err != nil {
		t.Fatal(err)
	}
	if err = native.Expunge(nil); err != nil {
		t.Fatalf("native EXPUNGE: %v", err)
	}
	gone := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef})
	if gone["exists"] != false {
		t.Fatal("native EXPUNGE not visible to browser")
	}
	// Keep the same authenticated connection while individual and parent grants
	// are revoked. PEEK must not avoid reauthorization just because it is read-only.
	if _, err = native.Select("Copies", false); err != nil {
		t.Fatal(err)
	}
	messageRef = copyRef
	seq = new(imap.SeqSet)
	seq.AddNum(1)
	fetch(false)
	patchGrant := func(table, ref string, permission int64) {
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/"+table+"/"+ref, admin,
			accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"permission": permission}), http.StatusOK)
	}
	patchGrant("mail", copyRef, 0)
	fetch(true)
	patchGrant("mail", copyRef, private)
	patchGrant("mail_box", copyBox, 0)
	fetch(true)
	if _, err = native.Status("Copies", []imap.StatusItem{imap.StatusMessages}); err == nil {
		t.Fatal("revoked mailbox STATUS succeeded")
	}
	patchGrant("mail_box", copyBox, int64(auth.UserPeek|auth.UserRead))
	if err = native.Unsubscribe("Copies"); err == nil {
		t.Fatal("read-only mailbox allowed subscription mutation")
	}
	patchGrant("mail_box", copyBox, private)
	patchGrant("mail_account", accountRef, 0)
	fetch(true)
	list(true)
}
