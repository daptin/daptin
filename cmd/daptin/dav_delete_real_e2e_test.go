package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDAVCalendarCollectionDeletionRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVCalendarCollectionDeletionRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-delete.db"))
}

func TestDAVCalendarCollectionDeletionPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVCalendarCollectionDeletionRealE2E(t, "postgres", dsn)
}

func runDAVCalendarCollectionDeletionRealE2E(t *testing.T, databaseType, connectionString string) {
	t.Helper()
	usedPorts := make(map[int]bool)
	smtpPort := freeTransportE2EPort(t, usedPorts)
	client := &http.Client{Timeout: 20 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: davE2EAccessSchema,
		})
		return base, process
	}
	firstURL, first := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
	first.stopProcess()
	base, process := start()
	defer process.stopProcess()
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "delete-calendar-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "delete-calendar-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing calendar owner: %s", principal.body)
	}
	mailServer := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": true, "listen_interface": fmt.Sprintf("127.0.0.1:%d", smtpPort),
		"max_size": 10000, "max_clients": 1, "xclient_on": false,
		"always_on_tls": false, "authentication_required": false,
	})
	mailAccount := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_account", map[string]interface{}{
		"username": "organizer@localhost", "password": "testpass123", "password_md5": "testpass123", "mail_server_id": mailServer,
	})
	certificate := accessGroupsE2ECreateRecord(t, client, base, adminToken, "certificate", map[string]interface{}{
		"hostname": "localhost", "issuer": "self",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/certificate/generate_self_certificate",
		adminToken, map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificate}}, http.StatusOK)
	connectedPath := "/caldav/" + owner[1] + "/calendars/connected/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+connectedPath, ownerToken, "", "", nil), http.StatusCreated)
	collection := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/collection?page%5Bsize%5D=100", ownerToken, nil, http.StatusOK)
	collectionID := davE2ECollectionReferenceID(t, collection, "connected")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+collectionID+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, mailAccount), nil), http.StatusNoContent)
	meeting := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:delete-calendar-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261018T120000Z\r\nDTEND:20261018T130000Z\r\nSEQUENCE:0\r\nSUMMARY:Delete calendar meeting\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE:mailto:first@example.test\r\nATTENDEE:mailto:second@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	meetingURL := base + connectedPath + "meeting.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, meetingURL, ownerToken, "text/calendar", meeting, nil), http.StatusCreated)
	ordinary := strings.Replace(meeting, "UID:delete-calendar-e2e", "UID:delete-calendar-ordinary-e2e", 1)
	ordinary = strings.Replace(ordinary, "ORGANIZER:mailto:organizer@localhost\r\nATTENDEE:mailto:first@example.test\r\nATTENDEE:mailto:second@example.test\r\n", "", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, base+connectedPath+"ordinary.ics", ownerToken,
		"text/calendar", ordinary, nil), http.StatusCreated)
	denied := davE2ERequest(client, http.MethodDelete, base+connectedPath, otherToken, "", "", nil)
	if denied.err != nil || denied.status == http.StatusNoContent {
		t.Fatalf("unrelated account deleted a connected calendar: %+v", denied)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, meetingURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+connectedPath, ownerToken, "", "", nil), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, meetingURL, ownerToken, "", "", nil), http.StatusNotFound)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+connectedPath+"ordinary.ics", ownerToken, "", "", nil), http.StatusNotFound)
	messages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	counts := make(map[string]int)
	for _, item := range messages {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "delete-calendar-e2e" {
			counts[fmt.Sprint(attrs["method"])]++
		}
	}
	if counts["REQUEST"] != 2 || counts["CANCEL"] != 2 {
		t.Fatalf("calendar deletion did not queue one cancellation per attendee: %v", counts)
	}
	attendeePrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", otherToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	attendeeOwner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(attendeePrincipal.body)
	if len(attendeeOwner) != 2 {
		t.Fatalf("missing attendee principal: %s", attendeePrincipal.body)
	}
	attendeeAccount := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_account", map[string]interface{}{
		"username": "attendee@localhost", "password": "testpass123", "password_md5": "testpass123", "mail_server_id": mailServer,
	})
	attendeePath := "/caldav/" + attendeeOwner[1] + "/calendars/meetings/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+attendeePath, otherToken, "", "", nil), http.StatusCreated)
	attendeeCollections := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/collection?page%5Bsize%5D=100", otherToken, nil, http.StatusOK)
	attendeeCollectionID := davE2ECollectionReferenceID(t, attendeeCollections, "meetings")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+attendeeCollectionID+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, attendeeAccount), nil), http.StatusNoContent)
	attendeeMeeting := strings.Replace(meeting, "UID:delete-calendar-e2e", "UID:delete-attendee-calendar-e2e", 1)
	attendeeMeeting = strings.Replace(attendeeMeeting, "ORGANIZER:mailto:organizer@localhost\r\nATTENDEE:mailto:first@example.test\r\nATTENDEE:mailto:second@example.test\r\n",
		"ORGANIZER:mailto:outside@example.test\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@localhost\r\n", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, base+attendeePath+"invitation.ics", otherToken,
		"text/calendar", attendeeMeeting, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+attendeePath, otherToken, "", "", nil), http.StatusNoContent)
	messages = accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	attendeeReplies := 0
	for _, item := range messages {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "delete-attendee-calendar-e2e" && attrs["method"] == "REPLY" &&
			strings.Contains(fmt.Sprint(attrs["icalendar"]), "PARTSTAT=DECLINED") {
			attendeeReplies++
		}
	}
	if attendeeReplies != 1 {
		t.Fatalf("attendee calendar deletion did not queue one declined reply: %d", attendeeReplies)
	}
	unconnectedPath := "/caldav/" + owner[1] + "/calendars/unconnected/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+unconnectedPath, ownerToken, "", "", nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, base+unconnectedPath+"ordinary.ics", ownerToken,
		"text/calendar", ordinary, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+unconnectedPath, ownerToken, "", "", nil), http.StatusNoContent)
	after := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(after) != len(messages) {
		t.Fatalf("unconnected calendar deletion queued scheduling mail: before=%d after=%d", len(messages), len(after))
	}
}
