package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/artpar/go-imap"
	imapclient "github.com/artpar/go-imap/client"
	"github.com/artpar/go-imap/commands"
	"github.com/daptin/daptin/server/auth"
	"github.com/google/uuid"
)

// This test connects to the actual STARTTLS IMAP listener of an isolated Daptin
// executable. Resource/action HTTP remains the setup and browser authority.
func TestMailNativeRoundTripRealE2E(t *testing.T) {
	requireRealE2E(t)
	options := mailProtocolDatabase(t)
	fixture := startDaptinE2E(t, options)
	admin := fixture.SignupAdmin(t)
	alice := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "native-alice")
	aliceRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "user_account", "email", "native-alice@test.local")
	private := int64(auth.UserCRUD | auth.UserExecute)
	const imapHostname = "localhost"
	seed := func(table string, values map[string]interface{}) string {
		ref := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, table, values)
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/"+table+"/"+ref, admin,
			accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"user_account_id": aliceRef, "permission": private}), http.StatusOK)
		return ref
	}
	serverRef := seed("mail_server", map[string]interface{}{"hostname": "native.test", "is_enabled": false, "listen_interface": "127.0.0.1:0", "max_size": 30000000, "max_clients": 1, "xclient_on": false, "always_on_tls": false, "authentication_required": true})
	accountRef := seed("mail_account", map[string]interface{}{"username": "native-alice@native.test", "password": "fixture-password", "password_md5": "fixture-password", "mail_server_id": serverRef})
	thunderbird := os.Getenv("DAPTIN_E2E_THUNDERBIRD_DRIVER")
	var publicPEM string
	if thunderbird != "" {
		attributes, root := thunderbirdFixtureCertificate(t, imapHostname)
		accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", attributes)
		publicPEM = root
	} else {
		cert := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", map[string]interface{}{"hostname": imapHostname, "issuer": "self"})
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPost, fixture.URL+"/action/certificate/generate_self_certificate", admin,
			map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": cert}}, http.StatusOK)
		certRecord := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/certificate/"+cert, admin, nil, http.StatusOK)
		publicPEM, _ = accessGroupsE2EFindString(certRecord, "certificate_pem")
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM([]byte(publicPEM)) {
		t.Fatal("fixture certificate could not be loaded")
	}
	tlsConfig := &tls.Config{RootCAs: roots, ServerName: imapHostname}
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
	configure("imap.hostname", imapHostname)
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
	// Verify the configured IMAP hostname against only this fixture's public
	// certificate. The test leaves the machine's root trust unchanged.
	native, err := imapclient.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, address)
	if err != nil {
		t.Fatalf("IMAP connect: %v", err)
	}
	native.Timeout = 15 * time.Second
	if thunderbird != "" {
		// The independent macOS app can take time to launch. Keep this
		// already-authenticated IMAP connection alive while it does.
		native.Timeout = 90 * time.Second
	}
	t.Cleanup(func() { _ = native.Terminate() })
	if err = native.StartTLS(tlsConfig); err != nil {
		t.Fatalf("STARTTLS: %v", err)
	}
	// The mail account username is the full address. Neither the local part
	// nor the Office sign-in identity names this mail account.
	for _, nonAccount := range []string{"native-alice", "native-alice@test.local"} {
		if err = native.Login(nonAccount, "fixture-password"); err == nil {
			t.Fatalf("IMAP accepted a username that does not identify the mail account: %q", nonAccount)
		}
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
	checkRoles := func(infos []*imap.MailboxInfo) {
		t.Helper()
		roles := map[string]string{
			"Draft": imap.DraftsAttr, "Sent": imap.SentAttr, "Archive": imap.ArchiveAttr,
			"Trash": imap.TrashAttr, "Spam": imap.JunkAttr,
		}
		for _, info := range infos {
			if role, found := roles[info.Name]; found {
				if !slices.Contains(info.Attributes, role) {
					t.Fatalf("IMAP folder %q omitted %s: attributes=%q", info.Name, role, info.Attributes)
				}
				delete(roles, info.Name)
			}
			if (info.Name == "INBOX" || info.Name == "Native/Draft") && slices.Contains(info.Attributes, imap.DraftsAttr) {
				t.Fatalf("Non-draft folder %q advertised the draft role", info.Name)
			}
		}
		if len(roles) != 0 {
			t.Fatalf("IMAP discovery omitted canonical folders: %v", roles)
		}
	}
	checkRoles(list(false))
	roleSubscriptions := make(chan *imap.MailboxInfo, 16)
	if err = native.Lsub("", "*", roleSubscriptions); err != nil {
		t.Fatalf("native LSUB: %v", err)
	}
	var subscriptions []*imap.MailboxInfo
	for folder := range roleSubscriptions {
		subscriptions = append(subscriptions, folder)
	}
	checkRoles(subscriptions)
	call := func(name, family string, input map[string]interface{}) map[string]interface{} {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail_account/"+name, alice,
			map[string]interface{}{"attributes": map[string]interface{}{family: input}}, http.StatusOK)
		return response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	}
	draft := map[string]interface{}{"account_ref": accountRef, "compose_key": uuid.NewString(), "write_key": uuid.NewString(),
		"content": map[string]interface{}{"subject": "Browser round trip", "text": "مرحبا नमस्ते こんにちは", "html": "<p>مرحبا नमस्ते こんにちは</p><img src=\"cid:inline%2F2026%25@native\">",
			"to": []map[string]interface{}{{"address": "recipient@example.test"}}, "bcc": []map[string]interface{}{{"address": "hidden@example.test"}},
			"in_reply_to": "<original@test>", "references": []string{"<root@test>", "<original@test>"},
			"parts": []map[string]interface{}{{"filename": "inline.png", "media_type": "image/png", "content_id": "inline/2026%@native", "base64": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aH2kAAAAASUVORK5CYII="}, {"filename": "résumé.txt", "media_type": "text/plain", "base64": "cHJvb2Y="}}}}
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
	checkNativeMove(t, native, fixture, admin, accountRef, call, source)
	checkNativeFlagIndependence(t, native, fixture, admin, accountRef, call)
	checkNativeFetchHydration(t, native, fixture, admin)
	checkNativeRawSourceFetch(t, native, fixture, admin, source)
	draftMailbox, err := native.Select("Draft", false)
	if err != nil {
		t.Fatalf("native SELECT: %v", err)
	}
	seq := new(imap.SeqSet)
	seq.AddNum(uid)
	checkNativeDestinationRecovery(t, native, seq, raw, func(bits int64) {
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail_account/"+accountRef, admin,
			accessGroupsE2ERecordPayload("mail_account", accountRef, map[string]interface{}{"permission": bits}), http.StatusOK)
	}, private)
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
	// An existing deleted message must stay in place when a readonly session
	// closes, even though writable CLOSE would expunge it.
	sentinel := []byte("From: native-alice@native.test\r\nSubject: Read-only CLOSE sentinel\r\n\r\nRetain this row.\r\n")
	if err = native.Append("Draft", []string{imap.DeletedFlag}, time.Now(), bytes.NewReader(sentinel)); err != nil {
		t.Fatalf("create readonly CLOSE sentinel: %v", err)
	}
	sentinelReference := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Read-only CLOSE sentinel")
	messageState := map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef}
	beforeRead := call("message_state", "message", messageState)
	if beforeRead["seen"] != false {
		beforeRead = call("message_flags", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef,
			"expected_box_ref": beforeRead["mail_box_id"], "expected_version": beforeRead["version"], "seen": false})
	}
	if beforeRead["seen"] != false {
		t.Fatal("readonly FETCH precondition: original draft must be unread")
	}
	readonly, err := native.Select("Draft", true)
	if err != nil {
		t.Fatalf("native EXAMINE: %v", err)
	}
	if !readonly.ReadOnly || len(readonly.PermanentFlags) != 0 {
		t.Errorf("EXAMINE advertised writable flag state: readOnly=%v permanent=%q", readonly.ReadOnly, readonly.PermanentFlags)
	}
	if readonly.Recent == 0 {
		t.Fatal("readonly polling precondition: the mailbox has no recent messages")
	}
	if err = native.Noop(); err != nil {
		t.Fatalf("read-only NOOP: %v", err)
	}
	polled, err := native.Select("Draft", true)
	if err != nil {
		t.Fatalf("EXAMINE after read-only NOOP: %v", err)
	}
	if polled.Recent != readonly.Recent {
		t.Errorf("read-only NOOP changed RECENT from %d to %d", readonly.Recent, polled.Recent)
	}
	nonPeek := &imap.BodySectionName{}
	read := make(chan *imap.Message, 2)
	if err = native.UidFetch(seq, []imap.FetchItem{nonPeek.FetchItem()}, read); err != nil {
		t.Fatalf("read-only UID FETCH: %v", err)
	}
	readMessage := <-read
	if readMessage == nil || readMessage.GetBody(nonPeek) == nil {
		t.Fatal("read-only FETCH omitted the message")
	}
	bodyBytes, err := io.ReadAll(readMessage.GetBody(nonPeek))
	if err != nil || !bytes.Equal(bodyBytes, raw) {
		t.Fatal("read-only FETCH did not return the same MIME")
	}
	afterRead := call("message_state", "message", messageState)
	if afterRead["seen"] != false {
		t.Error("read-only non-PEEK FETCH marked the draft Seen")
		// Keep the remainder of the baseline check independent of this failure.
		call("message_flags", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef,
			"expected_box_ref": afterRead["mail_box_id"], "expected_version": afterRead["version"], "seen": false})
	}
	if err = native.UidStore(seq, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.FlaggedFlag}, nil); err == nil {
		t.Error("read-only STORE changed the draft")
	}
	if err = native.Close(); err != nil {
		t.Fatalf("read-only CLOSE: %v", err)
	}
	retained := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": sentinelReference})
	if retained["exists"] != true || retained["deleted"] != true {
		t.Errorf("read-only CLOSE expunged the deleted message: %v", retained["exists"])
	}
	if _, err = native.Select("Draft", false); err != nil {
		t.Fatalf("native reselect after EXAMINE: %v", err)
	}
	if executable := os.Getenv("DAPTIN_E2E_MUTT_BINARY"); executable != "" || thunderbird != "" {
		message := map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef}
		clearFlags := func(state map[string]interface{}) {
			state = call("message_flags", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": messageRef,
				"expected_box_ref": state["mail_box_id"], "expected_version": state["version"], "seen": false, "flagged": false})
			if state["seen"] != false || state["flagged"] != false {
				t.Fatal("HTTP did not clear the existing flags")
			}
		}
		clearFlags(call("message_state", "message", message))
		var passed bool
		var thunderbirdSearchUID, thunderbirdArchiveUID, thunderbirdComposedUID uint32
		var archiveRef, archiveBox string
		var beforeArchive map[string]interface{}
		var archiveMIME []byte
		if thunderbird != "" {
			archiveMIME = []byte("From: native-alice@native.test\r\nSubject: Thunderbird archive fixture\r\n" +
				"Message-ID: <thunderbird-archive-fixture@test>\r\nMIME-Version: 1.0\r\n" +
				"Content-Type: multipart/mixed; boundary=archive-proof\r\n\r\n" +
				"--archive-proof\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\nMove kept the original message body.\r\n" +
				"--archive-proof\r\nContent-Type: application/octet-stream\r\n" +
				"Content-Disposition: attachment; filename=archived.bin\r\nContent-Transfer-Encoding: base64\r\n\r\nAP9wcm9vZg==\r\n--archive-proof--\r\n")
			if err := native.Append("INBOX", []string{imap.FlaggedFlag}, time.Now(), bytes.NewReader(archiveMIME)); err != nil {
				t.Fatalf("seed Thunderbird Inbox fixture: %v", err)
			}
			if err := native.Append("Archive", nil, time.Now(), strings.NewReader("From: native-alice@native.test\r\nSubject: Archive existing message\r\n\r\nKeep this message.\r\n")); err != nil {
				t.Fatalf("seed Thunderbird Archive: %v", err)
			}
			archiveRef = accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Thunderbird archive fixture")
			archiveBox = accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail_box", "name", "Archive")
			beforeArchive = call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": archiveRef})
			if beforeArchive["mail_box_id"] == archiveBox || !bytes.Equal(source(archiveRef), archiveMIME) {
				t.Fatal("Thunderbird archive fixture was not stored unchanged in its source mailbox")
			}
			passed = t.Run("desktop_thunderbird_starttls", func(t *testing.T) {
				thunderbirdSearchUID, thunderbirdArchiveUID, thunderbirdComposedUID = exerciseMailWithThunderbird(t, thunderbird, address, imapHostname, publicPEM)
			})
		} else {
			passed = t.Run("desktop_mutt_starttls", func(t *testing.T) {
				exerciseMailWithMutt(t, executable, address, imapHostname, publicPEM)
			})
		}
		if !passed {
			return
		}
		state := call("message_state", "message", message)
		if state["seen"] != true || state["flagged"] != true || state["uid"] != float64(uid) {
			t.Fatalf("desktop client flags on canonical UID %d: seen=%v flagged=%v uid=%v", uid, state["seen"], state["flagged"], state["uid"])
		}
		t.Log("Daptin HTTP confirmed the desktop client updated Seen and Flagged on the same canonical draft UID")
		clearFlags(state)
		if thunderbird != "" {
			archived := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": archiveRef})
			if archived["reference_id"] != archiveRef || archived["mail_box_id"] != archiveBox || archived["uid"] != float64(thunderbirdArchiveUID) ||
				archived["uid"] == beforeArchive["uid"] || archived["flagged"] != true || archived["deleted"] != false ||
				!bytes.Equal(source(archiveRef), archiveMIME) {
				t.Fatal("Office did not observe the original canonical message and MIME at Thunderbird's Archive UID")
			}
			t.Log("Daptin HTTP confirmed Thunderbird's Archive UID on the original reference, with the flagged state and exact multipart MIME")
			appended := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Thunderbird native draft")
			appendedState := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": appended})
			if appendedState["mail_box_id"] != state["mail_box_id"] || appendedState["uid"] == state["uid"] ||
				!bytes.Contains(source(appended), []byte("Thunderbird native draft body")) {
				t.Fatal("Thunderbird did not append a distinct draft to the canonical Draft mailbox")
			}
			if appendedState["uid"] != float64(thunderbirdSearchUID) {
				t.Fatalf("Thunderbird search returned UID %d, canonical draft UID is %v", thunderbirdSearchUID, appendedState["uid"])
			}
			body := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/unpack", alice,
				map[string]interface{}{"attributes": map[string]interface{}{"message_ref": appended}}, http.StatusOK)
			unpack := body.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
			attachmentFound := false
			for _, part := range unpack["parts"].([]interface{}) {
				fields := part.(map[string]interface{})
				if fields["filename"] == "tb-proof.bin" && fields["kind"] == "attachment" {
					data, _ := accessGroupsE2EFindString(fields, "contents")
					attachmentFound = data == "AP9wcm9vZg=="
				}
			}
			if !attachmentFound {
				t.Fatal("Office unpack did not recover the Thunderbird draft attachment")
			}
			revision, _ := accessGroupsE2EFindString(body, "revision")
			adopted := call("draft_save", "draft", map[string]interface{}{
				"account_ref": accountRef, "compose_key": uuid.NewString(), "write_key": uuid.NewString(),
				"draft_ref": appended, "expected_revision": revision,
				"content": map[string]interface{}{"subject": "Adopted Thunderbird draft", "text": "Edited through the Office draft contract"},
			})
			adoptedState := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": appended})
			if adopted["reference_id"] != appended || adoptedState["uid"].(float64) <= appendedState["uid"].(float64) || adoptedState["mail_box_id"] != appendedState["mail_box_id"] {
				t.Fatal("Office adoption changed the Thunderbird draft identity")
			}
			t.Log("Daptin HTTP confirmed the online-search UID and attachment bytes, then adopted the same Thunderbird draft reference and mailbox with a replacement UID")
			composed := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Thunderbird composer attachment")
			composedState := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": composed})
			if composedState["mail_box_id"] != state["mail_box_id"] || composedState["uid"] != float64(thunderbirdComposedUID) {
				t.Fatal("Office did not find the draft reopened by Thunderbird at the same canonical mailbox and UID")
			}
			composedBody := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/unpack", alice,
				map[string]interface{}{"attributes": map[string]interface{}{"message_ref": composed}}, http.StatusOK)
			composedUnpack := composedBody.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
			composedText, _ := composedUnpack["preferred_text_body"].(string)
			envelope := composedUnpack["envelope"].(map[string]interface{})
			for field, expected := range map[string]string{"to": "compose-recipient@example.test", "bcc": "compose-blind@example.test"} {
				addresses, _ := envelope[field].([]interface{})
				address, _ := accessGroupsE2EFindString(addresses, "address")
				if len(addresses) != 1 || address != expected {
					t.Fatalf("Thunderbird composer draft lost its %s recipient: %v", field, addresses)
				}
			}
			if !strings.Contains(composedText, "Native compose proof — こんにちは") {
				t.Fatalf("Office unpack lost the Thunderbird composer body: %q", composedText)
			}
			matches := 0
			for _, part := range composedUnpack["parts"].([]interface{}) {
				fields := part.(map[string]interface{})
				if fields["filename"] == "native-compose.bin" && fields["kind"] == "attachment" {
					data, _ := accessGroupsE2EFindString(fields, "contents")
					if data != base64.StdEncoding.EncodeToString([]byte("\x00\xffnative compose proof\r\n")) {
						t.Fatal("Office unpack changed the bytes of the Thunderbird composer attachment")
					}
					matches++
				}
			}
			if matches != 1 {
				t.Fatalf("Office unpack returned %d matches for the single Thunderbird composer attachment", matches)
			}
			t.Log("Daptin HTTP matched the reopened Thunderbird composer UID, multilingual body, To and Bcc, and exact binary attachment")
		}
	}
	for _, flag := range []string{imap.AnsweredFlag, imap.FlaggedFlag, imap.DeletedFlag, imap.SeenFlag, imap.DraftFlag} {
		if !slices.Contains(draftMailbox.Flags, flag) || !slices.Contains(draftMailbox.PermanentFlags, flag) {
			t.Fatalf("Draft did not advertise writable system flag %s: %v %v", flag, draftMailbox.Flags, draftMailbox.PermanentFlags)
		}
	}
	if slices.Contains(draftMailbox.PermanentFlags, imap.RecentFlag) || slices.Contains(draftMailbox.Flags, imap.TryCreateFlag) ||
		strings.Count(strings.Join(draftMailbox.PermanentFlags, " "), imap.TryCreateFlag) != 1 {
		t.Fatalf("Invalid advertised Draft capabilities: %v %v", draftMailbox.Flags, draftMailbox.PermanentFlags)
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
	if err = native.Create("Native/Draft"); err != nil {
		t.Fatalf("native CREATE ordinary nested Draft: %v", err)
	}
	checkRoles(list(false))
	checkNativeContentSearch(t, native, func(permission int64) {
		ref := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Header search fixture one")
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail/"+ref, admin,
			accessGroupsE2ERecordPayload("mail", ref, map[string]interface{}{"permission": permission}), http.StatusOK)
	}, private)
	checkNativeSimpleMultipart(t, native, func() (string, interface{}, interface{}) {
		ref := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Native mixed plain fixture")
		record := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/mail/"+ref+"?fields=has_attachment,body", alice, nil, http.StatusOK)
		body := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/unpack", alice,
			map[string]interface{}{"attributes": map[string]interface{}{"message_ref": ref}}, http.StatusOK)
		return ref, record, body
	}, source)
	if os.Getenv("DAPTIN_E2E_MAIL_SEARCH_SCALE") == "1" {
		checkNativeContentSearchScale(t, native)
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
	neighbor := []byte("From: native-alice@native.test\r\nSubject: Retained draft neighbor\r\n\r\nThis message must not move or change.\r\n")
	if err = native.Append("Native/2026", nil, nativeDate, bytes.NewReader(neighbor)); err != nil {
		t.Fatal(err)
	}
	neighborRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", "Retained draft neighbor")
	beforeEdit, err := native.Select("Native/2026", false)
	if err != nil {
		t.Fatal(err)
	}
	readDraftUID := func(uid uint32) []byte {
		t.Helper()
		requested := new(imap.SeqSet)
		requested.AddNum(uid)
		body := &imap.BodySectionName{Peek: true}
		messages := make(chan *imap.Message, 2)
		if err := native.UidFetch(requested, []imap.FetchItem{imap.FetchUid, body.FetchItem()}, messages); err != nil {
			t.Fatal(err)
		}
		got := <-messages
		if got == nil {
			return nil
		}
		literal := got.GetBody(body)
		if got.Uid != uid || literal == nil || <-messages != nil {
			t.Fatalf("unexpected response to UID FETCH %d", uid)
		}
		data, err := io.ReadAll(literal)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	oldUID := uint32(nativeState["uid"].(float64))
	if !bytes.Equal(readDraftUID(oldUID), nativeRaw) {
		t.Fatal("initial UID FETCH did not return the native draft before Office edit")
	}
	parsed := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/mail/unpack", alice, map[string]interface{}{"attributes": map[string]interface{}{"message_ref": nativeRef}}, http.StatusOK)
	revision, _ := accessGroupsE2EFindString(parsed, "revision")
	draft["draft_ref"], draft["compose_key"], draft["write_key"], draft["expected_revision"] = nativeRef, uuid.NewString(), uuid.NewString(), revision
	draft["content"].(map[string]interface{})["subject"] = "Adopted native draft"
	idling, idleRead := openMailIdleFixture(t, address, tlsConfig)
	adopted := call("draft_save", "draft", draft)
	adoptedState := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": nativeRef})
	_ = idling.SetDeadline(time.Now().Add(7 * time.Second))
	for _, want := range []string{"* 1 EXPUNGE\r\n", fmt.Sprintf("* %d EXISTS\r\n", beforeEdit.Messages), fmt.Sprintf("* %d RECENT\r\n", beforeEdit.Recent)} {
		if line, err := idleRead.ReadString('\n'); err != nil || line != want {
			t.Fatalf("passive IDLE draft replacement: got %q, want %q: %v", line, want, err)
		}
	}
	if _, err := io.WriteString(idling, "DONE\r\n"); err != nil {
		t.Fatal(err)
	}
	if line, err := idleRead.ReadString('\n'); err != nil || !strings.HasPrefix(line, "i OK ") {
		t.Fatalf("IDLE completion: %q %v", line, err)
	}
	if _, err := io.WriteString(idling, "n NOOP\r\n"); err != nil {
		t.Fatal(err)
	}
	if line, err := idleRead.ReadString('\n'); err != nil || !strings.HasPrefix(line, "n OK ") {
		t.Fatalf("NOOP after IDLE repeated updates or lost the parser boundary: %q %v", line, err)
	}
	if status, err := native.Status("Native/2026", []imap.StatusItem{imap.StatusRecent}); err != nil || beforeEdit.Recent == 0 || status.Recent != beforeEdit.Recent {
		t.Fatalf("read-only IDLE consumed Recent: %v, %v", status, err)
	}
	t.Log("Read-only IDLE announced the replacement without polling, kept Recent, and DONE stopped duplicate updates")
	func() {
		_ = idling.SetDeadline(time.Now().Add(7 * time.Second))
		if _, err := io.WriteString(idling, "r IDLE\r\n"); err != nil {
			t.Fatal(err)
		}
		if line, err := idleRead.ReadString('\n'); err != nil || !strings.HasPrefix(line, "+ ") {
			t.Fatalf("second IDLE continuation: %q %v", line, err)
		}
		permission := func(bits int64) {
			accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail_account/"+accountRef, admin,
				accessGroupsE2ERecordPayload("mail_account", accountRef, map[string]interface{}{"permission": bits}), http.StatusOK)
		}
		permission(0)
		defer permission(private)
		if line, err := idleRead.ReadString('\n'); err != nil || !strings.HasPrefix(line, "* BYE ") {
			t.Fatalf("revoked IDLE must close before releasing mailbox data: %q %v", line, err)
		}
		if line, err := idleRead.ReadString('\n'); err != io.EOF || line != "" {
			t.Fatalf("revoked IDLE connection did not close after BYE: %q %v", line, err)
		}
	}()
	t.Log("IDLE checked authorization on an unchanged mailbox and disconnected the revoked session without mailbox updates")
	// The database has renumbered its rows, but the client has not received
	// an EXPUNGE yet. Sequence 1 must never resolve to the former sequence 2.
	first, second := new(imap.SeqSet), new(imap.SeqSet)
	first.AddNum(1)
	second.AddRange(2, 0)
	if err := native.Store(first, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{imap.FlaggedFlag}, nil); err != nil {
		t.Fatal(err)
	}
	if adjacent := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": neighborRef}); adjacent["flagged"] != false {
		t.Error("stale sequence STORE changed flags on its adjacent message")
	}
	section := &imap.BodySectionName{Peek: true}
	missing := make(chan *imap.Message, 2)
	if err := native.Fetch(first, []imap.FetchItem{section.FetchItem()}, missing); err != nil || len(missing) != 0 {
		t.Fatalf("stale FETCH returned another message: %v, %d results", err, len(missing))
	}
	kept := make(chan *imap.Message, 2)
	if err := native.Fetch(second, []imap.FetchItem{section.FetchItem()}, kept); err != nil || len(kept) != 1 {
		t.Fatalf("FETCH of the surviving sequence did not return one message: %v, %d", err, len(kept))
	}
	msg := <-kept
	literal := msg.GetBody(section)
	if literal == nil {
		t.Fatal("FETCH omitted the surviving message literal")
	}
	data, err := io.ReadAll(literal)
	if err != nil || msg.SeqNum != 2 || !bytes.Equal(data, neighbor) {
		t.Fatal("the surviving UID lost its announced sequence or returned incorrect content")
	}
	criteria := &imap.SearchCriteria{Not: []*imap.SearchCriteria{{SeqNum: first}}}
	if got, err := native.Search(criteria); err != nil || !slices.Equal(got, []uint32{2}) {
		t.Fatalf("SEARCH negation used unannounced sequence numbers: %v, %v", got, err)
	}
	if err := native.Create("Stale sequence destination"); err != nil {
		t.Fatal(err)
	}
	if err := native.Copy(first, "Stale sequence destination"); err != nil {
		t.Fatal(err)
	}
	if destination, err := native.Status("Stale sequence destination", []imap.StatusItem{imap.StatusMessages}); err != nil || destination.Messages != 0 {
		t.Fatalf("COPY from a vanished sequence copied a different message: %v, %v", destination, err)
	}
	t.Log("Before NOOP, a vanished sequence could not FETCH, STORE or COPY a neighbor; the surviving sequence and nested SEARCH stayed stable")
	updates := make(chan imapclient.Update, 20)
	native.Updates = updates
	if err := native.Noop(); err != nil {
		t.Fatal(err)
	}
	var announced []string
	for len(updates) > 0 {
		switch update := (<-updates).(type) {
		case *imapclient.ExpungeUpdate:
			announced = append(announced, fmt.Sprintf("EXPUNGE %d", update.SeqNum))
		case *imapclient.MailboxUpdate:
			announced = append(announced, "MAILBOX UPDATE")
		}
	}
	if len(announced) != 3 || announced[0] != "EXPUNGE 1" || announced[1] != "MAILBOX UPDATE" || announced[2] != "MAILBOX UPDATE" || native.Mailbox().Messages != beforeEdit.Messages {
		t.Fatalf("selected draft replacement must announce EXPUNGE then EXISTS and RECENT without reselect: %v (messages %d)", announced, native.Mailbox().Messages)
	}
	if err := native.Noop(); err != nil || len(updates) != 0 {
		t.Fatalf("a second NOOP repeated draft replacement updates: %v, %d", err, len(updates))
	}
	native.Updates = nil
	afterEdit, err := native.Status("Native/2026", []imap.StatusItem{imap.StatusMessages, imap.StatusUidNext, imap.StatusUidValidity})
	if err != nil {
		t.Fatal(err)
	}
	if raw := readDraftUID(oldUID); raw != nil {
		t.Fatalf("old draft UID %d still resolves after the MIME edit (same original bytes: %v)", oldUID, bytes.Equal(raw, nativeRaw))
	}
	newUID := uint32(adoptedState["uid"].(float64))
	if adopted["reference_id"] != nativeRef || adopted["uid"] != adoptedState["uid"] || newUID != beforeEdit.UidNext || newUID <= oldUID || adoptedState["mail_box_id"] != nativeState["mail_box_id"] || adoptedState["flagged"] != true {
		t.Fatal("native draft adoption lost canonical identity/flags")
	}
	if beforeEdit.UidValidity != afterEdit.UidValidity || beforeEdit.Messages != afterEdit.Messages || afterEdit.UidNext != newUID+1 || !bytes.Equal(readDraftUID(newUID), source(nativeRef)) {
		t.Fatal("a draft edit did not replace only the edited message at a new UID")
	}
	replayed := call("draft_save", "draft", draft)
	if replayed["uid"] != adopted["uid"] || replayed["revision"] != adopted["revision"] {
		t.Fatal("replay of a successful draft edit allocated another UID or changed MIME")
	}
	if status, err := native.Status("Native/2026", []imap.StatusItem{imap.StatusUidNext}); err != nil || status.UidNext != afterEdit.UidNext {
		t.Fatalf("draft replay changed mailbox UIDNEXT: %v, %v", status, err)
	}
	t.Logf("NOOP announced EXPUNGE before EXISTS for Office draft replacement UID %d → %d, without reselect or replay; new UID returned exact MIME", oldUID, newUID)
	// Empty TEXT is a valid literal; it must not make FETCH omit the message.
	empty := []byte("From: native-alice@native.test\r\nTo: recipient@example.test\r\nSubject: Empty native body\r\n\r\n")
	if err = native.Append("Native/2026", nil, nativeDate, bytes.NewReader(empty)); err != nil {
		t.Fatal(err)
	}
	if _, err = native.Select("Native/2026", false); err != nil {
		t.Fatal(err)
	}
	emptySeq := new(imap.SeqSet)
	emptySeq.AddNum(afterEdit.UidNext)
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
	secure := tls.Client(bounded, tlsConfig)
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
	if _, err = io.WriteString(secure, "x EXAMINE iNbOx\r\n"); err != nil {
		t.Fatal(err)
	}
	sawEmptyPermanentFlags := false
	for {
		line, err := wireReader.ReadString('\n')
		if err != nil {
			t.Fatalf("wire EXAMINE: %v", err)
		}
		if strings.Contains(line, "[PERMANENTFLAGS ()]") {
			sawEmptyPermanentFlags = true
		}
		if strings.HasPrefix(line, "x ") {
			if !sawEmptyPermanentFlags || !strings.HasPrefix(line, "x OK [READ-ONLY]") {
				t.Fatalf("mixed-case INBOX EXAMINE did not explicitly advertise read-only flags: %q", line)
			}
			break
		}
	}
	_ = secure.SetDeadline(time.Now().Add(10 * time.Second))
	checkNativeFetchStructure(t, secure, wireReader, uid)
	checkNativeDigestFetch(t, native, secure, wireReader)
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
	beforeRevoke, err := native.Select("Copies", false)
	if err != nil {
		t.Fatal(err)
	}
	if beforeRevoke.ReadOnly || beforeRevoke.Recent == 0 {
		t.Fatal("polling revocation precondition: a writable mailbox with recent messages is required")
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
	if err = native.Noop(); err != nil {
		t.Errorf("NOOP after revoking selected mailbox writes: %v", err)
	}
	if err = native.Unsubscribe("Copies"); err == nil {
		t.Fatal("read-only mailbox allowed subscription mutation")
	}
	checkSeenAfterFetch := func(step string, peek, seen, flagUpdate bool) {
		t.Helper()
		section := &imap.BodySectionName{Peek: peek}
		messages := make(chan *imap.Message, 2)
		if err := native.UidFetch(seq, []imap.FetchItem{section.FetchItem()}, messages); err != nil {
			t.Errorf("%s: body FETCH failed with a current read grant: %v", step, err)
			return
		}
		got := <-messages
		if got == nil || got.GetBody(section) == nil {
			t.Errorf("%s: body omitted", step)
			return
		}
		body, err := io.ReadAll(got.GetBody(section))
		if err != nil || !bytes.Equal(body, raw) {
			t.Errorf("%s: body differs", step)
		}
		_, returnedFlags := got.Items[imap.FetchFlags]
		if returnedFlags != flagUpdate {
			t.Errorf("%s: body FETCH returned FLAGS = %v, want %v", step, returnedFlags, flagUpdate)
		}
		if flagUpdate && (!slices.Contains(got.Flags, imap.SeenFlag) || !slices.Contains(got.Flags, imap.FlaggedFlag) ||
			!strings.Contains(strings.ToLower(strings.Join(got.Flags, ",")), "project")) {
			t.Errorf("%s: body FETCH must return changed and retained flags, got %q", step, got.Flags)
		}
		// Issue a fresh read from storage. The Office operation-state action
		// requires account write authority, which this fixture revokes.
		flags := make(chan *imap.Message, 2)
		if err := native.UidFetch(seq, []imap.FetchItem{imap.FetchFlags}, flags); err != nil {
			t.Errorf("%s: FLAGS fetch failed: %v", step, err)
			return
		}
		observed := <-flags
		if observed == nil || slices.Contains(observed.Flags, imap.SeenFlag) != seen {
			t.Errorf("%s: persistent Seen state did not match %v", step, seen)
		}
	}
	checkSeenAfterFetch("mailbox write permission revoked on the selected connection", false, false, false)
	selected, err := native.Select("Copies", false)
	if err != nil {
		t.Fatal(err)
	}
	if !selected.ReadOnly || len(selected.PermanentFlags) != 0 {
		t.Errorf("SELECT with a read-only mailbox grant advertised writable state: %v %q", selected.ReadOnly, selected.PermanentFlags)
	}
	if selected.Recent != beforeRevoke.Recent {
		t.Errorf("NOOP after mailbox write revocation changed RECENT from %d to %d", beforeRevoke.Recent, selected.Recent)
	}
	checkSeenAfterFetch("SELECT with a read-only mailbox grant", false, false, false)
	patchGrant("mail_box", copyBox, private)
	patchGrant("mail_account", accountRef, int64(auth.UserPeek|auth.UserRead))
	selected, err = native.Select("Copies", false)
	if err != nil || !selected.ReadOnly || len(selected.PermanentFlags) != 0 {
		t.Errorf("SELECT with a read-only account grant did not select read-only: %v", err)
	}
	checkSeenAfterFetch("SELECT with a read-only account grant", false, false, false)
	patchGrant("mail_account", accountRef, private)
	selected, err = native.Select("Copies", false)
	if err != nil || selected.ReadOnly || !slices.Contains(selected.PermanentFlags, imap.SeenFlag) {
		t.Fatalf("SELECT after restoring writes: %v", err)
	}
	patchGrant("mail", copyRef, int64(auth.UserPeek|auth.UserRead))
	checkSeenAfterFetch("message write permission revoked within a writable mailbox", false, false, false)
	patchGrant("mail", copyRef, private)
	checkSeenAfterFetch("PEEK FETCH after restoring writes", true, false, false)
	checkSeenAfterFetch("non-PEEK FETCH after restoring writes", false, true, true)
	checkSeenAfterFetch("non-PEEK FETCH for an already Seen message", false, true, false)
	if err = native.Noop(); err != nil {
		t.Fatalf("writable NOOP after restoring grants: %v", err)
	}
	selected, err = native.Select("Copies", false)
	if err != nil || selected.Recent != 0 {
		t.Fatalf("writable NOOP did not consume RECENT: %v %v", selected, err)
	}
	checkRecentCleared := func(step string, reviewed bool) {
		t.Helper()
		ch := make(chan *imap.Message, 2)
		if err := native.UidFetch(seq, []imap.FetchItem{imap.FetchFlags}, ch); err != nil {
			t.Fatal(err)
		}
		observed := <-ch
		if observed == nil || slices.Contains(observed.Flags, imap.RecentFlag) ||
			!slices.Contains(observed.Flags, imap.FlaggedFlag) || !slices.Contains(observed.Flags, imap.SeenFlag) {
			t.Errorf("%s: unexpected FLAGS after consuming RECENT: %v", step, observed)
		}
		if observed != nil {
			keywords := strings.Split(strings.ToLower(strings.Join(observed.Flags, ",")), ",")
			if !slices.Contains(keywords, "project") || slices.Contains(keywords, "reviewed") != reviewed {
				t.Errorf("%s: unexpected keywords in %v", step, observed.Flags)
			}
		}
		uids, err := native.UidSearch(&imap.SearchCriteria{WithFlags: []string{imap.RecentFlag}})
		if err != nil || len(uids) != 0 {
			t.Errorf("%s: SEARCH RECENT returned %v, %v", step, uids, err)
		}
		state := call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": copyRef})
		flags := state["flags"].([]interface{})
		if slices.Contains(flags, interface{}(imap.RecentFlag)) {
			t.Errorf("%s: Office flags included consumed Recent: %v", step, flags)
		}
	}
	checkRecentCleared("after NOOP and fresh selection", false)
	if err := native.UidStore(seq, imap.FormatFlagsOp(imap.AddFlags, true), []interface{}{"Reviewed"}, nil); err != nil {
		t.Fatal(err)
	}
	checkRecentCleared("after unrelated STORE", true)
	selected, err = native.Select("Copies", false)
	if err != nil || selected.Recent != 0 {
		t.Errorf("unrelated STORE changed RECENT after polling: %v %v", selected, err)
	}
	if !t.Failed() {
		t.Log("Consumed Recent stayed absent from mailbox status, FETCH, SEARCH and Office flags after an unrelated STORE")
	}
	patchGrant("mail_account", accountRef, 0)
	fetch(true)
	list(true)
}

func checkNativeFetchHydration(t *testing.T, native *imapclient.Client, fixture *daptinE2EFixture, admin string) {
	t.Helper()
	const folder, damaged, count = "NativeFetchWindow", "Fetch damaged body", 36
	if err := native.Create(folder); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 10, 16, 10, 11, 12, 0, time.UTC)
	message := func(subject string) string {
		return "From: native-alice@native.test\r\nSubject: " + subject + "\r\n\r\nFetch window body.\r\n"
	}
	for i := 1; i <= count; i++ {
		if err := native.Append(folder, nil, date, strings.NewReader(message(fmt.Sprintf("Fetch window %d", i)))); err != nil {
			t.Fatal(err)
		}
	}
	if err := native.Append(folder, []string{imap.FlaggedFlag}, date, strings.NewReader(message(damaged))); err != nil {
		t.Fatal(err)
	}
	last := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", damaged)
	accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail/"+last, admin,
		accessGroupsE2ERecordPayload("mail", last, map[string]interface{}{"mail": "!!!!"}), http.StatusOK)
	if _, err := native.Select(folder, true); err != nil {
		t.Fatal(err)
	}
	all := new(imap.SeqSet)
	all.AddRange(1, 0)
	metadata := make(chan *imap.Message, count+2)
	if err := native.UidFetch(all, []imap.FetchItem{imap.FetchFast}, metadata); err != nil {
		t.Fatal(err)
	}
	var got []*imap.Message
	for entry := range metadata {
		got = append(got, entry)
	}
	if len(got) != count+1 {
		t.Errorf("UID FETCH FAST returned %d metadata records, want %d including the unreadable MIME record", len(got), count+1)
	} else {
		for i, entry := range got {
			subject := fmt.Sprintf("Fetch window %d", i+1)
			if i == count {
				subject = damaged
			}
			if entry.Uid != uint32(i+1) || entry.SeqNum != uint32(i+1) ||
				entry.Size != uint32(len(message(subject))) || !entry.InternalDate.Equal(date) ||
				slices.Contains(entry.Flags, imap.FlaggedFlag) != (i == count) {
				t.Errorf("UID FETCH FAST lost ordered UID, size, date or flags at message %d", i+1)
			}
		}
	}
	valid := new(imap.SeqSet)
	valid.AddRange(1, count)
	section := &imap.BodySectionName{Peek: true}
	bodies := make(chan *imap.Message, count+1)
	if err := native.UidFetch(valid, []imap.FetchItem{imap.FetchUid, section.FetchItem()}, bodies); err != nil {
		t.Fatal(err)
	}
	n := 0
	for entry := range bodies {
		n++
		body := entry.GetBody(section)
		if body == nil {
			t.Fatalf("missing BODY.PEEK[] at message %d", n)
		}
		data, err := io.ReadAll(body)
		if err != nil || entry.Uid != uint32(n) || string(data) != message(fmt.Sprintf("Fetch window %d", n)) {
			t.Fatalf("wrong ordered MIME at message %d: %v", n, err)
		}
	}
	if n != count {
		t.Fatalf("received %d MIME bodies, want %d", n, count)
	}
	one := new(imap.SeqSet)
	one.AddNum(count + 1)
	unreadable := make(chan *imap.Message, 1)
	if err := native.UidFetch(one, []imap.FetchItem{section.FetchItem()}, unreadable); err == nil || len(unreadable) != 0 {
		t.Error("unavailable MIME did not fail BODY.PEEK[] explicitly")
	}
	if _, err := native.Select(folder, false); err != nil {
		t.Fatal(err)
	}
	stored := make(chan *imap.Message, 1)
	if err := native.UidStore(one, imap.FormatFlagsOp(imap.AddFlags, false), []interface{}{imap.SeenFlag, imap.DeletedFlag}, stored); err != nil {
		t.Fatalf("STORE on unavailable MIME: %v", err)
	}
	flags := <-stored
	if flags == nil || flags.Uid != count+1 || !slices.Contains(flags.Flags, imap.SeenFlag) ||
		!slices.Contains(flags.Flags, imap.DeletedFlag) || !slices.Contains(flags.Flags, imap.FlaggedFlag) {
		t.Fatal("STORE did not return the new flags while preserving the star on unavailable MIME")
	}
	if err := native.Expunge(nil); err != nil {
		t.Fatalf("EXPUNGE unavailable MIME: %v", err)
	}
	remaining, err := native.UidSearch(&imap.SearchCriteria{})
	if err != nil || len(remaining) != count || slices.Contains(remaining, count+1) {
		t.Fatalf("EXPUNGE unavailable MIME affected the wrong messages: %v, %v", remaining, err)
	}
	if !t.Failed() {
		t.Log("UID FETCH FAST returned 37 metadata records including unavailable MIME; BODY.PEEK[] returned 36 exact ordered messages and rejected the unavailable MIME")
		t.Log("Native STORE returned Seen, Deleted and retained Flagged for unavailable MIME; EXPUNGE removed only that message")
	}
}

func checkNativeFlagIndependence(t *testing.T, native *imapclient.Client, fixture *daptinE2EFixture, admin, accountRef string,
	call func(string, string, map[string]interface{}) map[string]interface{}) {
	t.Helper()
	const folder = "IndependentFlags"
	const subject = "Independent IMAP flags fixture"
	if err := native.Create(folder); err != nil {
		t.Fatal(err)
	}
	if err := native.Append(folder, []string{imap.FlaggedFlag, "Project"}, time.Now(),
		strings.NewReader("From: native-alice@native.test\r\nSubject: "+subject+"\r\n\r\nUnread message.\r\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := native.Select(folder, false); err != nil {
		t.Fatal(err)
	}
	ref := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", subject)
	state := func() map[string]interface{} {
		return call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": ref})
	}
	set := new(imap.SeqSet)
	set.AddNum(uint32(state()["uid"].(float64)))
	check := func(label string, seen, deleted, flagged bool, keyword bool) {
		t.Helper()
		messages := make(chan *imap.Message, 2)
		if err := native.UidFetch(set, []imap.FetchItem{imap.FetchFlags}, messages); err != nil {
			t.Fatal(err)
		}
		message := <-messages
		if message == nil {
			t.Fatal("flag fixture disappeared before EXPUNGE")
		}
		got := strings.ToLower(strings.Join(message.Flags, ","))
		for flag, expected := range map[string]bool{
			imap.SeenFlag: seen, imap.DeletedFlag: deleted, imap.RecentFlag: true, imap.FlaggedFlag: flagged, "Project": keyword,
		} {
			if slices.Contains(strings.Split(got, ","), strings.ToLower(flag)) != expected {
				t.Errorf("%s: native flags %q, want %s=%v", label, message.Flags, flag, expected)
			}
		}
		web := state()
		if web["seen"] != seen || web["deleted"] != deleted || web["flagged"] != flagged {
			t.Errorf("%s: canonical state seen=%v deleted=%v flagged=%v", label, web["seen"], web["deleted"], web["flagged"])
		}
	}
	change := func(operation imap.FlagsOp, flags ...interface{}) {
		t.Helper()
		if err := native.UidStore(set, imap.FormatFlagsOp(operation, true), flags, nil); err != nil {
			t.Fatal(err)
		}
	}
	check("before STORE", false, false, true, true)
	change(imap.AddFlags, imap.DeletedFlag)
	check("after +FLAGS Deleted", false, true, true, true)
	change(imap.RemoveFlags, imap.DeletedFlag)
	check("after -FLAGS Deleted", false, false, true, true)
	change(imap.SetFlags, imap.DeletedFlag)
	check("after FLAGS replaced with Deleted", false, true, false, false)
	change(imap.AddFlags, imap.SeenFlag)
	check("after explicit +FLAGS Seen", true, true, false, false)
	change(imap.RemoveFlags, imap.SeenFlag)
	check("after explicit -FLAGS Seen", false, true, false, false)
	if !t.Failed() {
		t.Log("IMAP STORE preserved independent Seen, Deleted, Recent, Flagged and keyword state across add, remove and replacement")
	}
}

func checkNativeDestinationRecovery(t *testing.T, native *imapclient.Client, seq *imap.SeqSet, raw []byte, setGrant func(int64), private int64) {
	t.Helper()
	appendCommand := func(name string) imap.Commander {
		return &commands.Append{Mailbox: name, Message: bytes.NewReader(raw)}
	}
	first := new(imap.SeqSet)
	first.AddNum(1)
	copyCommand := func(name string) imap.Commander {
		return &commands.Copy{SeqSet: first, Mailbox: name}
	}
	uidCopyCommand := func(name string) imap.Commander {
		return &commands.Uid{Cmd: &commands.Copy{SeqSet: seq, Mailbox: name}}
	}
	checkNo := func(label string, command imap.Commander, tryCreate bool) {
		t.Helper()
		status, err := native.Execute(command, nil)
		if err != nil || status == nil || status.Type != imap.StatusRespNo || (status.Code == imap.CodeTryCreate) != tryCreate {
			t.Errorf("%s = %v, %v; want NO with TRYCREATE=%v", label, status, err, tryCreate)
		}
	}
	for _, op := range []struct {
		name    string
		command func(string) imap.Commander
	}{
		{"NativeMissingAppend", appendCommand},
		{"NativeMissingCopy", copyCommand},
		{"NativeMissingUIDCopy", uidCopyCommand},
	} {
		checkNo(op.name, op.command(op.name), true)
		if _, err := native.Status(op.name, []imap.StatusItem{imap.StatusMessages}); err == nil {
			t.Fatalf("%s implicitly created its destination", op.name)
		}
		if err := native.Create(op.name); err != nil {
			t.Fatal(err)
		}
		status, err := native.Execute(op.command(op.name), nil)
		if err != nil || status == nil || status.Type != imap.StatusRespOk {
			t.Fatalf("%s retry after explicit CREATE: %v, %v", op.name, status, err)
		}
		if dest, err := native.Status(op.name, []imap.StatusItem{imap.StatusMessages}); err != nil || dest.Messages != 1 {
			t.Fatalf("%s has the wrong message count after retry: %v, %v", op.name, dest, err)
		}
		if _, err := native.Select(op.name, true); err != nil {
			t.Fatal(err)
		}
		section := &imap.BodySectionName{Peek: true}
		messages := make(chan *imap.Message, 2)
		if err := native.Fetch(first, []imap.FetchItem{section.FetchItem()}, messages); err != nil {
			t.Fatal(err)
		}
		message := <-messages
		if message == nil || message.GetBody(section) == nil {
			t.Fatalf("%s retry did not store a readable MIME body", op.name)
		}
		got, err := io.ReadAll(message.GetBody(section))
		if err != nil || !bytes.Equal(got, raw) {
			t.Fatalf("%s retry did not preserve the original MIME", op.name)
		}
		if _, err = native.Select("Draft", false); err != nil {
			t.Fatal(err)
		}
		checkNo(op.name+" invalid destination", op.command("Native//Invalid"), false)
	}
	setGrant(int64(auth.UserPeek | auth.UserRead))
	defer setGrant(private)
	checkNo("APPEND without create authority", appendCommand("NativeMissingDenied"), false)
	checkNo("UID COPY without create authority", uidCopyCommand("NativeMissingDenied"), false)
	if !t.Failed() {
		t.Log("Native APPEND, COPY and UID COPY reported TRYCREATE only for valid missing destinations with create authority; explicit CREATE and retry preserved the MIME")
	}
}

func checkNativeMove(t *testing.T, native *imapclient.Client, fixture *daptinE2EFixture, admin, accountRef string,
	call func(string, string, map[string]interface{}) map[string]interface{}, source func(string) []byte) {
	t.Helper()
	const from, to = "NativeMoveSource", "NativeMoveTarget"
	for _, name := range []string{from, to} {
		if err := native.Create(name); err != nil {
			t.Fatal(err)
		}
	}
	uidSet := func(uids ...uint32) *imap.SeqSet {
		set := new(imap.SeqSet)
		set.AddNum(uids...)
		return set
	}
	state := func(ref string) map[string]interface{} {
		return call("message_state", "message", map[string]interface{}{"account_ref": accountRef, "message_ref": ref})
	}
	appendMessage := func(folder, subject string) (string, string) {
		raw := fmt.Sprintf("From: native-alice@native.test\r\nSubject: %s\r\n\r\nNative MOVE content.\r\n", subject)
		if err := native.Append(folder, []string{imap.FlaggedFlag, "MoveKeyword"}, time.Now(), strings.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
		return accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail", "subject", subject), raw
	}
	first, firstBody := appendMessage(from, "Native move first")
	middle, _ := appendMessage(from, "Native move stays")
	last, lastBody := appendMessage(from, "Native move last")
	appendMessage(to, "Native move existing target")
	fromRef := state(first)["mail_box_id"]
	toRef := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "mail_box", "name", to)
	if _, err := native.Select(from, false); err != nil {
		t.Fatal(err)
	}
	updates := make(chan imapclient.Update, 32)
	native.Updates = updates
	defer func() { native.Updates = nil }()
	checkExpunges := func(want ...uint32) {
		t.Helper()
		var got []uint32
		for len(updates) > 0 {
			if update, ok := (<-updates).(*imapclient.ExpungeUpdate); ok {
				got = append(got, update.SeqNum)
			}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("MOVE EXPUNGE sequence numbers = %v, want %v", got, want)
		}
	}
	if supported, err := native.Support("MOVE"); err != nil || !supported {
		t.Fatalf("MOVE capability: supported=%v error=%v", supported, err)
	}
	status, err := native.Execute(&commands.Move{SeqSet: uidSet(1), Mailbox: "MoveMissing"}, nil)
	if err != nil || status == nil || status.Type != imap.StatusRespNo || status.Code != imap.CodeTryCreate {
		t.Errorf("MOVE to a missing mailbox = %v, %v; want NO [TRYCREATE]", status, err)
	}
	if _, err := native.Select(from, true); err != nil {
		t.Fatal(err)
	}
	if err := native.UidMove(uidSet(1), to); err == nil {
		t.Error("EXAMINE permitted MOVE")
	}
	if _, err := native.Select(from, false); err != nil {
		t.Fatal(err)
	}
	private := int64(auth.UserCRUD | auth.UserExecute)
	setGrant := func(bits int64) {
		accessGroupsE2EAssertStatus(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/mail/"+last, admin,
			accessGroupsE2ERecordPayload("mail", last, map[string]interface{}{"permission": bits}), http.StatusOK)
	}
	setGrant(private &^ int64(auth.UserDelete))
	if err := native.UidMove(uidSet(1, 3), to); err == nil {
		t.Error("MOVE succeeded without delete permission on a selected message")
	}
	setGrant(private)
	checkExpunges()
	if state(first)["mail_box_id"] != fromRef || state(last)["mail_box_id"] != fromRef {
		t.Fatal("failed MOVE committed a partial move")
	}
	if err := native.UidMove(uidSet(1, 3), to); err != nil {
		t.Fatalf("advertised UID MOVE: %v", err)
	}
	checkExpunges(3, 1)
	for _, expected := range []struct {
		ref, body string
		uid       int
	}{{first, firstBody, 2}, {last, lastBody, 3}} {
		current := state(expected.ref)
		if current["mail_box_id"] != toRef || current["uid"] != float64(expected.uid) ||
			current["flagged"] != true || current["deleted"] != false || !bytes.Equal(source(expected.ref), []byte(expected.body)) {
			t.Fatal("UID MOVE failed to preserve Office reference, original MIME, or flags at the new mailbox UID")
		}
	}
	remaining, err := native.UidSearch(&imap.SearchCriteria{})
	if err != nil || !slices.Equal(remaining, []uint32{2}) || state(middle)["mail_box_id"] != fromRef {
		t.Fatalf("UID MOVE removed an unrelated message: %v, %v", remaining, err)
	}
	if err := native.Move(uidSet(1), to); err != nil {
		t.Fatalf("advertised sequence MOVE: %v", err)
	}
	checkExpunges(1)
	if current := state(middle); current["mail_box_id"] != toRef || current["uid"] != float64(4) {
		t.Fatal("sequence MOVE did not use the renumbered source sequence")
	}
	if _, err := native.Select(to, false); err != nil {
		t.Fatal(err)
	}
	if err := native.UidMove(uidSet(2), to); err != nil {
		t.Fatalf("MOVE within selected mailbox: %v", err)
	}
	checkExpunges(2)
	if got, err := native.UidSearch(&imap.SearchCriteria{}); err != nil || !slices.Equal(got, []uint32{1, 3, 4, 5}) || native.Mailbox().Messages != 4 {
		t.Fatalf("same-mailbox MOVE did not replace the source UID: %v, %v", got, err)
	}
	if current := state(first); current["uid"] != float64(5) || current["mail_box_id"] != toRef {
		t.Fatal("same-mailbox MOVE lost the Office message reference")
	}
	t.Log("Native UID MOVE and MOVE preserved canonical identities, assigned destination UIDs and reported source EXPUNGE sequence numbers")
}

func checkNativeSimpleMultipart(t *testing.T, native *imapclient.Client, read func() (string, interface{}, interface{}), source func(string) []byte) {
	t.Helper()
	const box = "Native/SimpleMixed"
	if err := native.Create(box); err != nil {
		t.Fatal(err)
	}
	raw := "From: native-alice@native.test\r\nTo: recipient@example.test\r\nSubject: Native mixed plain fixture\r\nDate: Mon, 16 Nov 2026 12:00:00 +0000\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=mixed\r\n\r\n--mixed\r\nContent-Type: text/plain; charset=ISO-8859-1\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\nPlain =E9mail body.\r\n--mixed\r\nContent-Type: text/plain; charset=x-unknown-fixture\r\n\r\nOpaqueTokenDoNotIndex\r\n--mixed\r\nContent-Type: text/plain\r\nContent-Transfer-Encoding: base64\r\n\r\nQ29ycnVwdFByZWZpeFRva2Vu!!!!\r\n--mixed\r\nContent-Type: application/octet-stream\r\nContent-Disposition: attachment; filename=proof.bin\r\nContent-Transfer-Encoding: base64\r\n\r\nAP9wcm9vZg==\r\n--mixed--\r\n"
	if err := native.Append(box, nil, time.Now(), strings.NewReader(raw)); err != nil {
		t.Fatalf("APPEND direct mixed text and attachment: %v", err)
	}
	ref, record, body := read()
	if !bytes.Equal(source(ref), []byte(raw)) {
		t.Fatal("APPEND changed original mixed MIME")
	}
	data := record.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if data["has_attachment"] != true && data["has_attachment"] != float64(1) {
		t.Fatalf("mixed message attachment marker missing: %v", data["has_attachment"])
	}
	if data["body"] != "Plain émail body." {
		t.Fatalf("mixed display body = %q", data["body"])
	}
	decoded, _ := accessGroupsE2EFindString(body, "preferred_text_body")
	if decoded != "Plain émail body." {
		t.Fatalf("mixed unpack body = %q", decoded)
	}
	if _, err := native.Select(box, true); err != nil {
		t.Fatal(err)
	}
	for term, count := range map[string]int{"body.": 1, "OpaqueTokenDoNotIndex": 0, "CorruptPrefixToken": 0} {
		result, err := native.UidSearch(&imap.SearchCriteria{Body: []string{term}})
		if err != nil || len(result) != count {
			t.Fatalf("native BODY search %q: %v, %v", term, result, err)
		}
	}
	fields := body.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	for _, value := range fields["parts"].([]interface{}) {
		part := value.(map[string]interface{})
		if part["filename"] == "proof.bin" && part["kind"] == "attachment" {
			payload, _ := accessGroupsE2EFindString(part, "contents")
			if payload != "AP9wcm9vZg==" {
				t.Fatalf("mixed attachment bytes = %q", payload)
			}
			return
		}
	}
	t.Fatal("mixed attachment was omitted by mail.unpack")
}

func checkNativeContentSearchScale(t *testing.T, native *imapclient.Client) {
	t.Helper()
	const box, count, samples = "Native/SearchScale", 320, 5
	if err := native.Create(box); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat(strings.Repeat("YWJj", 18)+"\r\n", 3800)
	nativeDate := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	var marked, headerMatches []uint32
	var wireBytes int
	for i := 1; i <= count; i++ {
		var flags []string
		header := "ordinary"
		if i%23 == 0 {
			flags = []string{imap.FlaggedFlag}
			marked = append(marked, uint32(i))
		}
		if i%13 == 0 {
			header = "match-50%_"
			headerMatches = append(headerMatches, uint32(i))
		}
		raw := fmt.Sprintf("From: native-alice@native.test\r\nSubject: Scale fixture %d\r\nMessage-ID: <scale-%d@native.test>\r\nX-Trace-Key: %s\r\n", i, i, header)
		if i%20 == 0 {
			raw += "MIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=scale\r\n\r\n--scale\r\nContent-Type: text/plain\r\n\r\nscale body needle\r\n--scale\r\nContent-Type: application/octet-stream\r\nContent-Transfer-Encoding: base64\r\n\r\n" + payload + "--scale--\r\n"
		} else {
			raw += "Content-Type: text/plain\r\n\r\nscale body needle\r\n" + strings.Repeat("scale padding ", 140)
		}
		if err := native.Append(box, flags, nativeDate, strings.NewReader(raw)); err != nil {
			t.Fatalf("APPEND %d: %v", i, err)
		}
		wireBytes += len(raw)
	}
	selected, err := native.Select(box, true)
	if err != nil || selected.Messages != count {
		t.Fatalf("EXAMINE scale mailbox: %v, %v", selected, err)
	}
	header := imap.NewSearchCriteria()
	header.Header.Add("X-Trace-Key", "match-50%_")
	bodyAndFlag := imap.NewSearchCriteria()
	bodyAndFlag.Body = []string{"scale body needle"}
	bodyAndFlag.WithFlags = []string{imap.FlaggedFlag}
	flagsOnly := imap.NewSearchCriteria()
	flagsOnly.WithFlags = []string{imap.FlaggedFlag}
	byUID := imap.NewSearchCriteria()
	byUID.Uid, err = imap.ParseSeqSet("299:320")
	if err != nil {
		t.Fatal(err)
	}
	byUID.Body = []string{"scale body needle"}
	wantUID := make([]uint32, 22)
	for i := range wantUID {
		wantUID[i] = uint32(299 + i)
	}
	t.Logf("IMAP search scale: messages=%d MIME_bytes=%d", count, wireBytes)
	for _, tt := range []struct {
		name     string
		criteria *imap.SearchCriteria
		want     []uint32
	}{
		{"HEADER", header, headerMatches},
		{"BODY_FLAGGED", bodyAndFlag, marked},
		{"BODY_UID_RANGE", byUID, wantUID},
		{"FLAGGED", flagsOnly, marked},
	} {
		times := make([]time.Duration, 0, samples)
		for i := 0; i <= samples; i++ {
			start := time.Now()
			found, err := native.UidSearch(tt.criteria)
			elapsed := time.Since(start)
			if err != nil || !slices.Equal(found, tt.want) {
				t.Fatalf("UID SEARCH %s: %v, %v; want %v", tt.name, found, err, tt.want)
			}
			if i > 0 {
				times = append(times, elapsed)
			}
		}
		slices.Sort(times)
		t.Logf("IMAP search scale: case=%s matches=%d median=%s samples=%v", tt.name, len(tt.want), times[len(times)/2], times)
	}
}

func checkNativeContentSearch(t *testing.T, native *imapclient.Client, grantFirst func(int64), private int64) {
	t.Helper()
	const box = "Native/Search"
	if err := native.Create(box); err != nil {
		t.Fatal(err)
	}
	other := "From: native-alice@native.test\r\nSubject: Outside search scope\r\nX-Trace-Key: duplicate-needle 50%_\r\n\r\n"
	if err := native.Append("Native/Draft", nil, time.Now(), strings.NewReader(other)); err != nil {
		t.Fatal(err)
	}
	fixtures := []string{
		"From: native-alice@native.test\r\nSubject: Header search fixture one\r\nX-Trace-Key: unrelated\r\nX-Trace-Key: Duplicate-Needle 50%_\r\nX-Empty:\r\nX-Name: =?UTF-8?Q?=C3=84rger?=\r\nContent-Type: multipart/alternative; boundary=search\r\n\r\n--search\r\nContent-Type: text/plain\r\n\r\nplain text only\r\n--search\r\nContent-Type: text/html; charset=UTF-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n<p>html-only =C3=84rger</p>\r\n--search--\r\n",
		"From: native-alice@native.test\r\nSubject: Header search fixture two\r\nX-Trace-Key: duplicate-needle 500X\r\n\r\nX-Empty: this is body text, not a header\r\n",
	}
	for idx, raw := range fixtures {
		var flags []string
		if idx == 1 {
			flags = []string{imap.FlaggedFlag}
		}
		if err := native.Append(box, flags, time.Now(), strings.NewReader(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := native.Select(box, true); err != nil {
		t.Fatal(err)
	}
	header := func(name, value string) *imap.SearchCriteria {
		c := imap.NewSearchCriteria()
		c.Header.Add(name, value)
		return c
	}
	check := func(label string, uid bool, criteria *imap.SearchCriteria, want ...uint32) {
		t.Helper()
		search := native.Search
		if uid {
			search = native.UidSearch
		}
		got, err := search(criteria)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("IMAP SEARCH %s: got %v (%v), want %v", label, got, err, want)
		}
	}
	check("repeated literal header", true, header("x-trace-key", "DUPLICATE-NEEDLE 50%_"), 1)
	check("present empty header", false, header("X-Empty", ""), 1)
	check("absent header", true, header("X-Absent", ""))
	check("decoded Unicode header", true, header("X-Name", "ärger"), 1)
	text := imap.NewSearchCriteria()
	text.Text = []string{"x-trace-key: duplicate-needle"}
	check("TEXT with arbitrary header", true, text, 1, 2)
	body := imap.NewSearchCriteria()
	body.Body = []string{"html-only ärger"}
	check("BODY with decoded HTML alternative", true, body, 1)
	flag := imap.NewSearchCriteria()
	flag.WithFlags = []string{imap.FlaggedFlag}
	mixed := imap.NewSearchCriteria()
	mixed.Or = append(mixed.Or, [2]*imap.SearchCriteria{flag, header("X-Empty", "")})
	mixed.Not = append(mixed.Not, header("X-Trace-Key", "500X"))
	check("OR/NOT and flag", true, mixed, 1)
	unseen := imap.NewSearchCriteria()
	unseen.WithoutFlags = []string{imap.SeenFlag}
	check("SEARCH leaves Seen unchanged", true, unseen, 1, 2)
	grantFirst(0)
	check("revoked row omitted from TEXT", true, text, 2)
	check("revoked row omitted from HEADER", true, header("X-Empty", ""))
	grantFirst(private)
	check("restored row visible", true, header("X-Empty", ""), 1)
}

func checkNativeFetchStructure(t *testing.T, conn io.Writer, reader *bufio.Reader, uid uint32) {
	t.Helper()
	wire := imap.NewReader(reader)
	command := func(tag, command string) []imap.Resp {
		t.Helper()
		if _, err := fmt.Fprintf(conn, "%s %s\r\n", tag, command); err != nil {
			t.Fatal(err)
		}
		var data []imap.Resp
		for {
			response, err := imap.ReadResp(wire)
			if err != nil {
				t.Fatalf("%s: %v", command, err)
			}
			if status, ok := response.(*imap.StatusResp); ok && status.Tag == tag {
				if status.Type != imap.StatusRespOk {
					t.Errorf("%s: unexpected %s: %s", command, status.Type, status.Info)
				}
				return data
			}
			data = append(data, response)
		}
	}
	command("s0", "SELECT Draft")
	test := func(tag, fetch string, want ...imap.FetchItem) {
		t.Helper()
		commandLine := fmt.Sprintf("UID FETCH %d %s", uid, fetch)
		items := map[imap.FetchItem]interface{}{}
		messages := 0
		for _, response := range command(tag, commandLine) {
			name, fields, ok := imap.ParseNamedResp(response)
			if !ok || name != "FETCH" || len(fields) != 2 {
				continue
			}
			messages++
			list, ok := fields[1].([]interface{})
			if !ok || len(list)%2 != 0 {
				t.Fatalf("%s: invalid FETCH items %#v", tag, fields)
			}
			for i := 0; i < len(list); i += 2 {
				item, ok := list[i].(string)
				if !ok {
					t.Fatalf("%s: invalid FETCH item name %#v", tag, list[i])
				}
				items[imap.FetchItem(item)] = list[i+1]
			}
		}
		if messages != 1 {
			t.Errorf("%s: %s returned %d message responses, want 1", tag, commandLine, messages)
			return
		}
		for _, key := range append(want, imap.FetchUid) {
			item, ok := items[key]
			if !ok {
				t.Errorf("%s: %s omitted %s", tag, commandLine, key)
				continue
			}
			if key == imap.FetchUid {
				got, err := imap.ParseNumber(item)
				if err != nil || got != uid {
					t.Errorf("%s: UID = %v, want %d", tag, item, uid)
				}
			}
			if key != imap.FetchBody && key != imap.FetchBodyStructure {
				continue
			}
			data, ok := item.([]interface{})
			if !ok {
				t.Errorf("%s: %s has no MIME structure", tag, key)
				continue
			}
			structure := new(imap.BodyStructure)
			if err := structure.Parse(data); err != nil {
				t.Errorf("%s: %s: %v", tag, key, err)
				continue
			}
			wantExtended := key == imap.FetchBodyStructure
			if !strings.EqualFold(structure.MIMEType, "multipart") || len(structure.Parts) < 2 {
				t.Errorf("%s: %s omitted multipart MIME", tag, key)
			}
			foundCID := false
			structure.Walk(func(path []int, part *imap.BodyStructure) bool {
				if part.Extended != wantExtended {
					t.Errorf("%s: %s at part %v extension state = %t, want %t", tag, key, path, part.Extended, wantExtended)
				}
				if wantExtended && strings.EqualFold(part.MIMEType, "multipart") && strings.EqualFold(part.MIMESubType, "related") && part.Params["type"] != "text/html" {
					t.Errorf("%s: BODYSTRUCTURE lost multipart/related's HTML root type: %v", tag, part.Params)
				}
				if strings.Trim(part.Id, "<>") == "inline/2026%@native" && strings.EqualFold(part.MIMEType, "image") {
					foundCID = true
					if wantExtended && part.Disposition != "inline" {
						t.Errorf("%s: BODYSTRUCTURE lost inline disposition", tag)
					}
				}
				return true
			})
			if !foundCID {
				t.Errorf("%s: %s lost the nested inline CID image", tag, key)
			}
		}
	}
	test("s1", "BODY", imap.FetchBody)
	test("s2", "FULL", imap.FetchFlags, imap.FetchInternalDate, imap.FetchRFC822Size, imap.FetchEnvelope, imap.FetchBody)
	test("s3", "BODYSTRUCTURE", imap.FetchBodyStructure)
	test("s4", "(BODY BODYSTRUCTURE)", imap.FetchBody, imap.FetchBodyStructure)
	command("s5", "EXAMINE Draft")
	test("s6", "(BODYSTRUCTURE BODY)", imap.FetchBodyStructure, imap.FetchBody)
}
