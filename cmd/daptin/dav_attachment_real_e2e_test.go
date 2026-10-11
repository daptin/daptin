package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

var davAttachmentE2ESchema = strings.Replace(davE2EAccessSchema, "  - TableName: collection", `    Columns:
      - Name: attachments
        ColumnName: attachments
        DataType: longblob
        ColumnType: file.*
        IsNullable: true
        IsForeignKey: true
        ForeignKeyData:
          DataSource: cloud_store
          Namespace: dav-attachment-e2e
          KeyName: calendar-files
  - TableName: collection`, 1)

func TestDAVManagedAttachmentRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := map[int]bool{}
	port := freeTransportE2EPort(t, usedPorts)
	httpsPort := freeTransportE2EPort(t, usedPorts)
	olricPort := freeTransportE2EPortPair(t, usedPorts)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	root := t.TempDir()
	options := transportE2EDaptinOptions{databaseType: "sqlite3", connectionString: filepath.Join(t.TempDir(), "dav-attachments.db"), olricPort: olricPort, schema: davAttachmentE2ESchema}
	if dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN"); dsn != "" {
		options.databaseType = "postgres"
		options.connectionString = dsn
	}
	process := startTransportE2EDaptin(t, port, httpsPort, base, options)
	client := &http.Client{Timeout: 30 * time.Second}
	admin := accessGroupsE2ESignupSigninAdmin(t, client, base)
	accessGroupsE2ECreateRecord(t, client, base, admin, "cloud_store", map[string]interface{}{
		"name": "dav-attachment-e2e", "store_type": "local", "store_provider": "local", "root_path": root, "store_parameters": "{}",
	})
	ftpE2ESetConfig(t, client, base, admin, "caldav.enable", "true")
	process.stopProcess()
	process = startTransportE2EDaptin(t, port, httpsPort, base, options)
	defer process.stopProcess()
	owner := accessGroupsE2ESignupSigninUser(t, client, base, admin, "attachment-owner")
	other := accessGroupsE2ESignupSigninUser(t, client, base, admin, "attachment-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", owner,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	ownerRef := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(ownerRef) != 2 {
		t.Fatalf("missing calendar principal: %s", principal.body)
	}
	for _, discoveryPath := range []string{"/caldav/", "/caldav/" + ownerRef[1] + "/"} {
		response := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, base+discoveryPath, owner, "", "", nil), http.StatusNoContent)
		if !strings.Contains(response.header.Get("DAV"), "calendar-managed-attachments-no-recurrence") {
			t.Fatalf("%s did not advertise attachments: %s", discoveryPath, response.header.Get("DAV"))
		}
	}
	optionsResponse := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, base+"/caldav/"+ownerRef[1]+"/calendars/", owner, "", "", nil), http.StatusNoContent)
	if !strings.Contains(optionsResponse.header.Get("DAV"), "calendar-managed-attachments-no-recurrence") {
		t.Fatalf("configured calendar home did not advertise attachments: %s", optionsResponse.header.Get("DAV"))
	}
	calendarPath := base + "/caldav/" + ownerRef[1] + "/calendars/attachments/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", calendarPath, owner, "", "", nil), http.StatusCreated)
	event := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:attachment-1\r\nDTSTAMP:20261009T000000Z\r\nDTSTART:20261020T120000Z\r\nDTEND:20261020T130000Z\r\nSUMMARY:Attachments\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	eventURL := calendarPath + "event.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, owner, "text/calendar", event, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, "REPORT", calendarPath, owner, "application/xml",
		`<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	before := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, owner, "", "", nil), http.StatusOK)
	denied := davE2ERequest(client, http.MethodPost, eventURL+"?action=attachment-add", other, "text/plain", "private", http.Header{"Content-Disposition": {`attachment; filename="agenda.txt"`}})
	if denied.err != nil || denied.status == http.StatusCreated {
		t.Fatalf("unshared user added an attachment: %+v", denied)
	}
	result := davE2EExpect(t, davE2ERequest(client, http.MethodPost, eventURL+"?action=attachment-add", owner,
		"text/plain", "first agenda", http.Header{"Content-Disposition": {`attachment; filename="agenda.txt"`}, "If-Match": {before.header.Get("ETag")}}), http.StatusCreated)
	id := result.header.Get("Cal-Managed-ID")
	if id == "" || result.header.Get("ETag") == before.header.Get("ETag") {
		t.Fatalf("attachment response omitted managed ID or new ETag: %+v", result.header)
	}
	stored := filepath.Join(root, "calendar-files", accessGroupsE2EFindResourceID(t, client, base, admin, "calendar", "rpath", strings.TrimPrefix(eventURL, base)), id)
	storedBytes, err := os.ReadFile(stored)
	if err != nil || string(storedBytes) != "first agenda" {
		t.Fatalf("attachment bytes not stored under event: %q, %v", storedBytes, err)
	}
	updated := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, owner, "", "", nil), http.StatusOK)
	if !strings.Contains(updated.body, "MANAGED-ID="+id) || !strings.Contains(updated.body, "FILENAME=agenda.txt") {
		t.Fatalf("event omitted managed attachment: %s", updated.body)
	}
	download := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL+"?attachment="+id, owner, "", "", nil), http.StatusOK)
	if download.body != "first agenda" {
		t.Fatalf("attachment download = %q", download.body)
	}
	denied = davE2ERequest(client, http.MethodGet, eventURL+"?attachment="+id, other, "", "", nil)
	if denied.err != nil || denied.status == http.StatusOK {
		t.Fatalf("unshared user downloaded attachment: %+v", denied)
	}
	stale := davE2ERequest(client, http.MethodPost, eventURL+"?action=attachment-add", owner,
		"text/plain", "stale", http.Header{"Content-Disposition": {`attachment; filename="agenda.txt"`}, "If-Match": {before.header.Get("ETag")}})
	if stale.err != nil || stale.status != http.StatusPreconditionFailed {
		t.Fatalf("stale attachment upload: %+v", stale)
	}
	replaced := davE2EExpect(t, davE2ERequest(client, http.MethodPost,
		eventURL+"?action=attachment-update&managed-id="+id, owner, "text/plain", "revised agenda",
		http.Header{"Content-Disposition": {`attachment; filename="agenda-v2.txt"`}, "If-Match": {updated.header.Get("ETag")}}), http.StatusOK)
	newID := replaced.header.Get("Cal-Managed-ID")
	if newID == "" || newID == id {
		t.Fatalf("attachment update did not rotate managed ID: %q", newID)
	}
	if old := davE2ERequest(client, http.MethodGet, eventURL+"?attachment="+id, owner, "", "", nil); old.err != nil || old.status != http.StatusNotFound {
		t.Fatalf("replaced attachment remained accessible: %+v", old)
	}
	if _, err := os.Stat(stored); !os.IsNotExist(err) {
		t.Fatalf("replaced attachment remained on disk: %v", err)
	}
	if current := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL+"?attachment="+newID, owner, "", "", nil), http.StatusOK); current.body != "revised agenda" {
		t.Fatalf("updated attachment bytes = %q", current.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPost,
		eventURL+"?action=attachment-remove&managed-id="+newID, owner, "", "",
		http.Header{"If-Match": {replaced.header.Get("ETag")}}), http.StatusNoContent)
	if removed := davE2ERequest(client, http.MethodGet, eventURL+"?attachment="+newID, owner, "", "", nil); removed.err != nil || removed.status != http.StatusNotFound {
		t.Fatalf("removed attachment remained accessible: %+v", removed)
	}
	mailServer := accessGroupsE2ECreateRecord(t, client, base, admin, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": false, "listen_interface": "127.0.0.1:0",
		"max_size": 10000, "max_clients": 1, "xclient_on": false,
		"always_on_tls": false, "authentication_required": false,
	})
	mailAccount := accessGroupsE2ECreateRecord(t, client, base, admin, "mail_account", map[string]interface{}{
		"username": "attachment-organizer@localhost", "password": "testpass123", "password_md5": "testpass123", "mail_server_id": mailServer,
	})
	certificate := accessGroupsE2ECreateRecord(t, client, base, admin, "certificate", map[string]interface{}{
		"hostname": "localhost", "issuer": "self",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/certificate/generate_self_certificate", admin,
		map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificate}}, http.StatusOK)
	collections := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/collection?page%5Bsize%5D=100", owner, nil, http.StatusOK)
	collectionID := davE2ECollectionReferenceID(t, collections, "attachments")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+collectionID+"/relationships/scheduling_mail_account_id", admin,
		"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, mailAccount), nil), http.StatusNoContent)
	scheduled := strings.Replace(event, "UID:attachment-1", "UID:attachment-scheduled", 1)
	recipient := "external@example.test"
	if os.Getenv("DAPTIN_TEST_MAILPIT_URL") != "" {
		recipient = "external@localhost"
	}
	scheduled = strings.Replace(scheduled, "SUMMARY:Attachments\r\n", "SUMMARY:Attachments\r\nORGANIZER:mailto:attachment-organizer@localhost\r\nATTENDEE:mailto:"+recipient+"\r\n", 1)
	scheduledURL := calendarPath + "scheduled.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, scheduledURL, owner, "text/calendar", scheduled, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, scheduledURL+"?action=attachment-add", owner,
		"text/plain", "external agenda", http.Header{"Content-Disposition": {`attachment; filename="external-agenda.txt"`}}), http.StatusCreated)
	outbox := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/outbox?included_relations=mail&page%5Bsize%5D=100", admin, nil, http.StatusOK))
	if len(outbox) != 2 {
		t.Fatalf("attachment update did not queue a second invitation: %d", len(outbox))
	}
	foundFile := false
	for _, item := range outbox {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		encoded, _ := attrs["mail"].(string)
		raw, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("queued invitation is not decodable: %v", err)
		}
		if strings.Contains(string(raw), "external-agenda.txt") && strings.Contains(string(raw), base64.StdEncoding.EncodeToString([]byte("external agenda"))) {
			foundFile = true
		}
	}
	if !foundFile {
		t.Fatal("external invitation did not include the attachment bytes")
	}
	if receiverURL := os.Getenv("DAPTIN_TEST_MAILPIT_URL"); receiverURL != "" {
		davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/outbox/process_outbox",
			admin, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
		for _, row := range outbox {
			id := row.(map[string]interface{})["id"].(string)
			processed := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/outbox/"+id,
				admin, nil, http.StatusOK)
			attrs := processed.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
			if attrs["sent"] != true && attrs["sent"] != float64(1) {
				t.Fatalf("SMTP did not accept invitation %s: sent=%v error=%v", id, attrs["sent"], attrs["last_error"])
			}
		}
		response, err := client.Get(receiverURL + "/api/v1/messages?limit=10")
		if err != nil {
			t.Fatal(err)
		}
		var received struct {
			Total    int `json:"total"`
			Messages []struct {
				ID string `json:"ID"`
			} `json:"messages"`
		}
		err = json.NewDecoder(response.Body).Decode(&received)
		response.Body.Close()
		if err != nil || response.StatusCode != http.StatusOK || received.Total != 2 {
			t.Fatalf("SMTP receiver has %d invitations, status=%d, error=%v", received.Total, response.StatusCode, err)
		}
		foundDeliveredFile := false
		for _, message := range received.Messages {
			raw, err := client.Get(receiverURL + "/api/v1/message/" + message.ID + "/raw")
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(raw.Body)
			raw.Body.Close()
			if err != nil || raw.StatusCode != http.StatusOK {
				t.Fatalf("could not read received invitation %s: status=%d error=%v", message.ID, raw.StatusCode, err)
			}
			if strings.Contains(string(body), "external-agenda.txt") && strings.Contains(string(body), base64.StdEncoding.EncodeToString([]byte("external agenda"))) {
				foundDeliveredFile = true
			}
		}
		if !foundDeliveredFile {
			t.Fatal("SMTP receiver did not receive the attachment bytes")
		}
	}
	withAttachment := davE2EExpect(t, davE2ERequest(client, http.MethodGet, scheduledURL, owner, "", "", nil), http.StatusOK)
	calendar, err := ical.NewDecoder(strings.NewReader(withAttachment.body)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	managedID := calendar.Events()[0].Props.Get("ATTACH").Params.Get("MANAGED-ID")
	calendar.Events()[0].Props.SetText("SUMMARY", "Attachments revised")
	var encoded bytes.Buffer
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		t.Fatal(err)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, scheduledURL, owner, "text/calendar", encoded.String(),
		http.Header{"If-Match": {withAttachment.header.Get("ETag")}}), http.StatusCreated)
	retained := davE2EExpect(t, davE2ERequest(client, http.MethodGet, scheduledURL+"?attachment="+managedID, owner, "", "", nil), http.StatusOK)
	if retained.body != "external agenda" {
		t.Fatalf("ordinary event edit lost attachment: %s", retained.body)
	}
	current := davE2EExpect(t, davE2ERequest(client, http.MethodGet, scheduledURL, owner, "", "", nil), http.StatusOK)
	calendar, err = ical.NewDecoder(strings.NewReader(current.body)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	calendar.Events()[0].Props.Del("ATTACH")
	encoded.Reset()
	if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
		t.Fatal(err)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, scheduledURL, owner, "text/calendar", encoded.String(),
		http.Header{"If-Match": {current.header.Get("ETag")}}), http.StatusCreated)
	if removed := davE2ERequest(client, http.MethodGet, scheduledURL+"?attachment="+managedID, owner, "", "", nil); removed.err != nil || removed.status != http.StatusNotFound {
		t.Fatalf("PUT removal retained attachment: %+v", removed)
	}
	moveSource := calendarPath + "move-source.ics"
	moveTarget := calendarPath + "move-target.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, moveSource, owner, "text/calendar",
		strings.Replace(event, "UID:attachment-1", "UID:attachment-move", 1), nil), http.StatusCreated)
	moveUpload := davE2EExpect(t, davE2ERequest(client, http.MethodPost, moveSource+"?action=attachment-add", owner,
		"text/plain", "move bytes", http.Header{"Content-Disposition": {`attachment; filename="move.txt"`}}), http.StatusCreated)
	moveID := moveUpload.header.Get("Cal-Managed-ID")
	davE2EExpect(t, davE2ERequest(client, "COPY", moveSource, owner, "", "", http.Header{"Destination": {calendarPath + "copy.ics"}}), http.StatusConflict)
	davE2EExpect(t, davE2ERequest(client, "MOVE", moveSource, owner, "", "", http.Header{"Destination": {moveTarget}}), http.StatusCreated)
	moved := davE2EExpect(t, davE2ERequest(client, http.MethodGet, moveTarget, owner, "", "", nil), http.StatusOK)
	if !strings.Contains(moved.body, "move-target.ics?attachment=") || strings.Contains(moved.body, "move-source.ics?attachment=") {
		t.Fatalf("MOVE retained old attachment URI: %s", moved.body)
	}
	if file := davE2EExpect(t, davE2ERequest(client, http.MethodGet, moveTarget+"?attachment="+moveID, owner, "", "", nil), http.StatusOK); file.body != "move bytes" {
		t.Fatalf("moved attachment bytes = %q", file.body)
	}
	movedID := accessGroupsE2EFindResourceID(t, client, base, admin, "calendar", "rpath", strings.TrimPrefix(moveTarget, base))
	deleted := davE2ERequest(client, http.MethodDelete, moveTarget, owner, "", "", nil)
	if deleted.err != nil || deleted.status != http.StatusNoContent {
		t.Fatalf("event deletion failed: %+v", deleted)
	}
	if afterDelete := davE2ERequest(client, http.MethodGet, moveTarget, owner, "", "", nil); afterDelete.err != nil || afterDelete.status == http.StatusOK {
		t.Fatalf("event still exists after DELETE: %+v", afterDelete)
	}
	if _, err := os.Stat(filepath.Join(root, "calendar-files", movedID, moveID)); !os.IsNotExist(err) {
		t.Fatalf("deleted event retained attachment bytes: %v", err)
	}
}
