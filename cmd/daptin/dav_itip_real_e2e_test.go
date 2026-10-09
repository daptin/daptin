package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/daptin/go-webdav/caldav"
	"github.com/emersion/go-ical"
)

type davSchedulingClientAuth struct {
	client *http.Client
	token  string
}

func (auth davSchedulingClientAuth) Do(request *http.Request) (*http.Response, error) {
	authenticated := request.Clone(request.Context())
	authenticated.Header.Set("Authorization", "Bearer "+auth.token)
	return auth.client.Do(authenticated)
}

func TestDAVConnectedCalendarQueuesInvitationsRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVConnectedCalendarSchedulingRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-itip.db"))
}

func TestDAVConnectedCalendarSchedulingPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVConnectedCalendarSchedulingRealE2E(t, "postgres", dsn)
}

func TestDAVSchedulingExternalFailureRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav-external-failure.db")
	client := &http.Client{Timeout: 30 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: "sqlite3", connectionString: databasePath, olricPort: olricPort, schema: davE2EAccessSchema,
		})
		return base, process
	}
	firstURL, first := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
	first.stopProcess()
	base, process := start()
	defer process.stopProcess()
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "itip-external-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "itip-external-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing calendar owner: %s", principal.body)
	}
	mailServer := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": false, "listen_interface": "127.0.0.1:0",
		"max_size": 10000, "max_clients": 1, "xclient_on": false,
		"always_on_tls": false, "authentication_required": false,
	})
	mailAccount := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_account", map[string]interface{}{
		"username": "sender@localhost", "password": "testpass123", "password_md5": "testpass123", "mail_server_id": mailServer,
	})
	certificate := accessGroupsE2ECreateRecord(t, client, base, adminToken, "certificate", map[string]interface{}{
		"hostname": "localhost", "issuer": "self",
	})
	accessGroupsE2ERequestJSON(t, client, http.MethodPost, base+"/action/certificate/generate_self_certificate",
		adminToken, map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificate}}, http.StatusOK)
	calendarPath := "/caldav/" + owner[1] + "/calendars/external/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+calendarPath, ownerToken, "", "", nil), http.StatusCreated)
	collections := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/collection?page%5Bsize%5D=100", ownerToken, nil, http.StatusOK)
	collectionID := davE2ECollectionReferenceID(t, collections, "external")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+collectionID+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, mailAccount), nil), http.StatusNoContent)
	autoOptions := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, base+calendarPath, ownerToken, "", "", http.Header{"Origin": {"http://localhost"}}), http.StatusNoContent)
	if !strings.Contains(autoOptions.header.Get("DAV"), "calendar-auto-schedule") {
		t.Fatalf("single connected calendar did not advertise automatic scheduling: %s", autoOptions.header.Get("DAV"))
	}
	event := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:itip-external-failure\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261017T120000Z\r\nDTEND:20261017T130000Z\r\nSEQUENCE:0\r\nSUMMARY:External failure\r\nORGANIZER:mailto:sender@localhost\r\nATTENDEE:mailto:nobody@nonexistent.invalid\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, base+calendarPath+"event.ics", ownerToken,
		"text/calendar", event, nil), http.StatusCreated)
	messages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", ownerToken, nil, http.StatusOK))
	if len(messages) != 1 {
		t.Fatalf("missing outbound scheduling record: %#v", messages)
	}
	messageID := messages[0].(map[string]interface{})["id"].(string)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/outbox/process_outbox",
		adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/cal_mail/reconcile_delivery",
		adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
	pending := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+calendarPath+"event.ics", ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(pending.body, "SCHEDULE-STATUS=1.0") {
		t.Fatalf("retryable SMTP failure changed pending delivery state: %s", pending.body)
	}
	deniedReconcile := davE2ERequest(client, http.MethodPost, base+"/action/cal_mail/reconcile_delivery",
		ownerToken, "application/json", `{"attributes":{}}`, nil)
	if deniedReconcile.err != nil || deniedReconcile.status == http.StatusOK {
		t.Fatalf("calendar owner ran system delivery reconciliation: %+v", deniedReconcile)
	}
	statusBody := fmt.Sprintf(`{"attributes":{"collection_id":"%s","message_id":"%s"}}`, collectionID, messageID)
	status := davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/scheduling_status",
		ownerToken, "application/json", statusBody, nil), http.StatusOK)
	if !strings.Contains(status.body, `"retry_count":1`) || !strings.Contains(status.body, "last_error") {
		t.Fatalf("external delivery failure is not visible to calendar owner: %s", status.body)
	}
	denied := davE2ERequest(client, http.MethodPost, base+"/action/collection/scheduling_status",
		otherToken, "application/json", statusBody, nil)
	if denied.err != nil {
		t.Fatal(denied.err)
	}
	if denied.status == http.StatusOK {
		t.Fatalf("other user read calendar delivery status: %s", denied.body)
	}
	outboxRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/outbox?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(outboxRows) != 1 {
		t.Fatalf("expected one failed outbox attempt, got %d", len(outboxRows))
	}
	outboxID := outboxRows[0].(map[string]interface{})["id"].(string)
	rescheduled := strings.Replace(event, "SEQUENCE:0", "SEQUENCE:1", 1)
	rescheduled = strings.Replace(rescheduled, "SUMMARY:External failure", "SUMMARY:External failure updated", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, base+calendarPath+"event.ics", ownerToken,
		"text/calendar", rescheduled, nil), http.StatusCreated)
	newPending := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+calendarPath+"event.ics", ownerToken, "", "", nil), http.StatusOK)
	outboxRows = accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/outbox?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(outboxRows) != 2 {
		t.Fatalf("rescheduling did not queue a new attempt: %#v", outboxRows)
	}
	newOutboxID := outboxRows[0].(map[string]interface{})["id"].(string)
	if newOutboxID == outboxID {
		newOutboxID = outboxRows[1].(map[string]interface{})["id"].(string)
	}
	// The first SMTP attempt has failed. Set the worker's terminal retry
	// state through the resource API to exercise reconciliation without
	// waiting for the production backoff schedule.
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/outbox/"+outboxID,
		adminToken, accessGroupsE2ERecordPayload("outbox", outboxID, map[string]interface{}{"retry_count": 5}), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/cal_mail/reconcile_delivery",
		adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
	stale := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+calendarPath+"event.ics", ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(stale.body, "SCHEDULE-STATUS=1.0") || stale.header.Get("ETag") != newPending.header.Get("ETag") {
		t.Fatalf("older delivery attempt replaced newer pending state: %s", stale.body)
	}
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/outbox/"+newOutboxID,
		adminToken, accessGroupsE2ERecordPayload("outbox", newOutboxID, map[string]interface{}{"retry_count": 5}), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/cal_mail/reconcile_delivery",
		adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
	failed := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+calendarPath+"event.ics", ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(failed.body, "SCHEDULE-STATUS=5.1") || failed.header.Get("ETag") == newPending.header.Get("ETag") ||
		failed.header.Get("Schedule-Tag") != newPending.header.Get("Schedule-Tag") {
		t.Fatalf("terminal SMTP failure did not update only delivery status: %s", failed.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/cal_mail/reconcile_delivery",
		adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
	failedAgain := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+calendarPath+"event.ics", ownerToken, "", "", nil), http.StatusOK)
	if failedAgain.header.Get("ETag") != failed.header.Get("ETag") {
		t.Fatalf("delivery reconciliation was not idempotent: %s", failedAgain.body)
	}
}

func runDAVConnectedCalendarSchedulingRealE2E(t *testing.T, databaseType, connectionString string) {
	usedPorts := make(map[int]bool)
	smtpPort := freeTransportE2EPort(t, usedPorts)
	if os.Getenv("DAPTIN_TEST_SMTP_PORT_25") == "1" {
		smtpPort = 25
		t.Setenv("SSL_CERT_FILE", filepath.Join(t.TempDir(), "smtp-root.pem"))
	}
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
	token := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "itip-owner")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing owner principal: %s", principal.body)
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
	accessGroupsE2ERequestJSON(t, client, http.MethodPost,
		base+"/action/certificate/generate_self_certificate", adminToken,
		map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificate}}, http.StatusOK)
	if smtpPort == 25 {
		certificateRow := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/certificate/"+certificate,
			adminToken, nil, http.StatusOK).(map[string]interface{})["data"].(map[string]interface{})
		publicPEM, _ := certificateRow["attributes"].(map[string]interface{})["certificate_pem"].(string)
		if !strings.Contains(publicPEM, "BEGIN CERTIFICATE") {
			t.Fatalf("generated SMTP certificate is unavailable for trust setup: %#v", certificateRow)
		}
		if err := os.WriteFile(os.Getenv("SSL_CERT_FILE"), []byte(publicPEM), 0600); err != nil {
			t.Fatal(err)
		}
	}
	calendarPath := "/caldav/" + owner[1] + "/calendars/connected/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+calendarPath, token, "", "", nil), http.StatusCreated)
	unconnectedPath := "/caldav/" + owner[1] + "/calendars/unconnected/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+unconnectedPath, token, "", "", nil), http.StatusCreated)
	collection := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/collection?page%5Bsize%5D=100", token, nil, http.StatusOK)
	collectionID := davE2ECollectionReferenceID(t, collection, "connected")
	scheduleProperties := `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-user-address-set/><C:schedule-inbox-URL/><C:schedule-outbox-URL/></D:prop></D:propfind>`
	beforeConnect := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+owner[1]+"/", token, "application/xml", scheduleProperties, nil), http.StatusMultiStatus)
	if strings.Contains(beforeConnect.body, "mailto:organizer@localhost") || !strings.Contains(beforeConnect.body, "404 Not Found") {
		t.Fatalf("unconnected principal advertises scheduling: %s", beforeConnect.body)
	}
	relation := fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, mailAccount)
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+collectionID+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", relation, nil), http.StatusNoContent)
	afterConnect := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+owner[1]+"/", token, "application/xml", scheduleProperties, nil), http.StatusMultiStatus)
	if !strings.Contains(afterConnect.body, "mailto:organizer@localhost") || !strings.Contains(afterConnect.body, "/schedule-inbox/") || !strings.Contains(afterConnect.body, "/schedule-outbox/") {
		t.Fatalf("connected principal lacks scheduling discovery: %s", afterConnect.body)
	}
	options := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, base+"/caldav/"+owner[1]+"/", token, "", "", nil), http.StatusNoContent)
	if strings.Contains(options.header.Get("DAV"), "calendar-auto-schedule") {
		t.Fatalf("incomplete automatic processing was advertised: %s", options.header.Get("DAV"))
	}
	calendarClient, err := caldav.NewClient(davSchedulingClientAuth{client: client, token: token}, base+"/caldav/")
	if err != nil {
		t.Fatal(err)
	}
	clientPrincipal, err := calendarClient.FindCurrentUserPrincipal(context.Background())
	if err != nil || clientPrincipal != "/caldav/"+owner[1]+"/" {
		t.Fatalf("CalDAV client could not discover principal: %q, %v", clientPrincipal, err)
	}
	clientHome, err := calendarClient.FindCalendarHomeSet(context.Background(), clientPrincipal)
	if err != nil || clientHome != "/caldav/"+owner[1]+"/calendars/" {
		t.Fatalf("CalDAV client could not discover home: %q, %v", clientHome, err)
	}
	clientCalendars, err := calendarClient.FindCalendars(context.Background(), clientHome)
	if err != nil || len(clientCalendars) != 2 {
		t.Fatalf("CalDAV client could not list calendars: %d, %v", len(clientCalendars), err)
	}
	ordinary := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:itip-later-invite-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261011T120000Z\r\nDTEND:20261011T130000Z\r\nSUMMARY:Ordinary meeting\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	ordinaryURL := base + calendarPath + "later-invite.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, ordinaryURL, token, "text/calendar", ordinary, nil), http.StatusCreated)
	invited := strings.Replace(ordinary, "SUMMARY:Ordinary meeting\r\n", "SUMMARY:Ordinary meeting\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;SCHEDULE-AGENT=NONE:mailto:guest@example.test\r\n", 1)
	forgedInvite := strings.Replace(invited, "ORGANIZER:mailto:organizer@localhost", "ORGANIZER:mailto:forged@example.test", 1)
	forgedInvite = strings.Replace(forgedInvite, "ATTENDEE;SCHEDULE-AGENT=NONE:mailto:guest@example.test", "ATTENDEE:mailto:organizer@localhost\r\nATTENDEE;SCHEDULE-AGENT=NONE:mailto:guest@example.test", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, ordinaryURL, token, "text/calendar", forgedInvite, nil), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, ordinaryURL, token, "text/calendar", invited, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, ordinaryURL, token, "text/calendar", forgedInvite, nil), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, ordinaryURL, token, "", "", nil), http.StatusNoContent)
	create := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:itip-outbound-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261012T120000Z\r\nDTEND:20261012T130000Z\r\nSEQUENCE:0\r\nSUMMARY:Meeting\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:guest@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	objectURL := base + calendarPath + "meeting.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, base+unconnectedPath+"meeting.ics", token, "text/calendar", create, nil), http.StatusCreated)
	unconnectedEvent := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+unconnectedPath+"meeting.ics", token, "", "", nil), http.StatusOK)
	if unconnectedEvent.header.Get("Schedule-Tag") != "" {
		t.Fatalf("unconnected event advertises scheduling: %s", unconnectedEvent.header.Get("Schedule-Tag"))
	}
	emptyMessages := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)
	if rows := accessGroupsE2EDataArray(t, emptyMessages); len(rows) != 0 {
		t.Fatalf("unconnected calendar queued invitations: %#v", rows)
	}
	invalidURL := base + calendarPath + "invalid.ics"
	missingProductID := strings.Replace(create, "PRODID:-//Daptin//EN\r\n", "", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, invalidURL, token,
		"text/calendar", missingProductID, nil), http.StatusBadRequest)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, invalidURL, token, "", "", nil), http.StatusNotFound)
	if rows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)); len(rows) != 0 {
		t.Fatalf("invalid calendar queued invitations: %#v", rows)
	}
	clientEvent, err := ical.NewDecoder(strings.NewReader(create)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := calendarClient.PutCalendarObject(context.Background(), calendarPath+"meeting.ics", clientEvent); err != nil {
		t.Fatalf("CalDAV client could not create scheduled event: %v", err)
	}
	if stored, err := calendarClient.GetCalendarObject(context.Background(), calendarPath+"meeting.ics"); err != nil || stored.Data == nil {
		t.Fatalf("CalDAV client could not read scheduled event: %v", err)
	} else if status := stored.Data.Events()[0].Props.Values("ATTENDEE")[0].Params.Get("SCHEDULE-STATUS"); status != "1.0" {
		t.Fatalf("queued invitation status = %q, want pending 1.0", status)
	}
	freeBusyRequest := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VFREEBUSY\r\nUID:itip-freebusy-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261012T000000Z\r\nDTEND:20261013T000000Z\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE:mailto:organizer@localhost\r\nEND:VFREEBUSY\r\nEND:VCALENDAR\r\n"
	freeBusy := davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/caldav/"+owner[1]+"/schedule-outbox/", token, "text/calendar", freeBusyRequest, nil), http.StatusOK)
	if !strings.Contains(freeBusy.body, "20261012T120000Z/20261012T130000Z") || strings.Contains(freeBusy.body, "SUMMARY:Meeting") {
		t.Fatalf("scheduling outbox did not return private-safe busy time: %s", freeBusy.body)
	}
	outboxPrivileges := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+owner[1]+"/schedule-outbox/", token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:supported-privilege-set/><D:current-user-privilege-set/></D:prop></D:propfind>`,
		http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	currentStart := strings.Index(outboxPrivileges.body, "<D:current-user-privilege-set>")
	currentEnd := strings.Index(outboxPrivileges.body, "</D:current-user-privilege-set>")
	if currentStart < 0 || currentEnd <= currentStart {
		t.Fatalf("outbox has no current privilege set: %s", outboxPrivileges.body)
	}
	currentPrivileges := outboxPrivileges.body[currentStart:currentEnd]
	for _, privilege := range []string{"schedule-send-invite", "schedule-send-reply", "schedule-send-freebusy"} {
		if !strings.Contains(outboxPrivileges.body, privilege) || !strings.Contains(currentPrivileges, privilege) {
			t.Fatalf("outbox does not grant %s: %s", privilege, outboxPrivileges.body)
		}
	}
	inboxPrivileges := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+owner[1]+"/schedule-inbox/", token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:supported-privilege-set/><D:current-user-privilege-set/></D:prop></D:propfind>`,
		http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	for _, privilege := range []string{"schedule-deliver-invite", "schedule-deliver-reply", "schedule-query-freebusy"} {
		if !strings.Contains(inboxPrivileges.body, privilege) {
			t.Fatalf("inbox does not support %s: %s", privilege, inboxPrivileges.body)
		}
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/caldav/"+owner[1]+"/schedule-outbox/", token, "text/calendar", strings.Replace(freeBusyRequest, "ORGANIZER:mailto:organizer@localhost", "ORGANIZER:mailto:forged@example.test", 1), nil), http.StatusForbidden)
	assertMessages := func(want int, method string) {
		t.Helper()
		messages := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)
		rows := accessGroupsE2EDataArray(t, messages)
		if len(rows) != want {
			t.Fatalf("got %d iTIP messages, want %d: %#v", len(rows), want, messages)
		}
		found := false
		recipient := "guest@example.test"
		if method == "REPLY" {
			recipient = "organizer@localhost"
		}
		for _, item := range rows {
			attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
			if attrs["method"] == method && attrs["recipient_address"] == recipient && strings.Contains(fmt.Sprint(attrs["icalendar"]), "METHOD:"+method) {
				for _, parameter := range []string{"SCHEDULE-STATUS", "SCHEDULE-AGENT", "SCHEDULE-FORCE-SEND"} {
					if strings.Contains(fmt.Sprint(attrs["icalendar"]), parameter) {
						t.Fatalf("outbound %s exposes server-only %s: %#v", method, parameter, attrs)
					}
				}
				found = true
			}
		}
		if !found {
			t.Fatalf("missing %s iTIP message: %#v", method, rows)
		}
	}
	assertMessages(1, "REQUEST")
	transferPath := "/caldav/" + owner[1] + "/calendars/transfer/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+transferPath, token, "", "", nil), http.StatusCreated)
	transferredURL := base + transferPath + "transferred.ics"
	davE2EExpect(t, davE2ERequest(client, "COPY", objectURL, token, "", "",
		http.Header{"Destination": {transferredURL}}), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, "MOVE", transferredURL, token, "", "",
		http.Header{"Destination": {base + transferPath + "renamed.ics"}}), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+transferPath+"renamed.ics", token, "", "", nil), http.StatusNoContent)
	assertMessages(1, "REQUEST")
	serverStatus := davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusOK)
	forgedStatus := strings.Replace(serverStatus.body, "SCHEDULE-STATUS=1.0", "SCHEDULE-STATUS=1.2", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, "text/calendar", forgedStatus, nil), http.StatusCreated)
	retainedStatus := davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(retainedStatus.body, "SCHEDULE-STATUS=1.0") || strings.Contains(retainedStatus.body, "SCHEDULE-STATUS=1.2") {
		t.Fatalf("client changed server-managed delivery status: %s", retainedStatus.body)
	}
	assertMessages(1, "REQUEST")
	for _, agent := range []string{"CLIENT", "NONE"} {
		managed := strings.Replace(create, "UID:itip-outbound-e2e", "UID:itip-agent-"+strings.ToLower(agent), 1)
		managed = strings.Replace(managed, "ATTENDEE;PARTSTAT=NEEDS-ACTION:", "ATTENDEE;PARTSTAT=NEEDS-ACTION;SCHEDULE-AGENT="+agent+":", 1)
		managedURL := base + calendarPath + "agent-" + strings.ToLower(agent) + ".ics"
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, managedURL, token, "text/calendar", managed, nil), http.StatusCreated)
		assertMessages(1, "REQUEST")
		davE2EExpect(t, davE2ERequest(client, http.MethodDelete, managedURL, token, "", "", nil), http.StatusNoContent)
		assertMessages(1, "REQUEST")
	}
	selfAttendee := strings.Replace(create, "UID:itip-outbound-e2e", "UID:itip-self-attendee-e2e", 1)
	selfAttendee = strings.Replace(selfAttendee, "ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:guest@example.test",
		"ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION;SCHEDULE-AGENT=NONE:mailto:guest@example.test", 1)
	selfURL := base + calendarPath + "self-attendee.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, selfURL, token, "text/calendar", selfAttendee, nil), http.StatusCreated)
	assertMessages(1, "REQUEST")
	withoutSelf := strings.Replace(selfAttendee, "ATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:organizer@localhost\r\n", "", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, selfURL, token, "text/calendar", withoutSelf, nil), http.StatusCreated)
	assertMessages(1, "REQUEST")
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, selfURL, token, "text/calendar", selfAttendee, nil), http.StatusCreated)
	assertMessages(1, "REQUEST")
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, selfURL, token, "", "", nil), http.StatusNoContent)
	assertMessages(1, "REQUEST")
	initialMessages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", token, nil, http.StatusOK))
	if len(initialMessages) != 1 {
		t.Fatalf("calendar owner cannot see queued invitation: %#v", initialMessages)
	}
	firstMessageID := initialMessages[0].(map[string]interface{})["id"].(string)
	statusBody := fmt.Sprintf(`{"attributes":{"collection_id":"%s","message_id":"%s"}}`, collectionID, firstMessageID)
	status := davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/scheduling_status",
		token, "application/json", statusBody, nil), http.StatusOK)
	if !strings.Contains(status.body, "retry_count") || !strings.Contains(status.body, "sent") {
		t.Fatalf("calendar owner cannot inspect queued delivery: %s", status.body)
	}
	outboxRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/outbox?included_relations=mail&page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(outboxRows) != 1 {
		t.Fatalf("expected one queued iMIP mail, got %d", len(outboxRows))
	}
	outboxAttrs := outboxRows[0].(map[string]interface{})["attributes"].(map[string]interface{})
	mailValue, ok := outboxAttrs["mail"].(string)
	if !ok {
		t.Fatalf("queued mail contents unavailable: %#v", outboxAttrs["mail"])
	}
	rawOutbox, err := base64.StdEncoding.DecodeString(mailValue)
	if err != nil || !strings.Contains(string(rawOutbox), "DKIM-Signature:") || !strings.Contains(string(rawOutbox), "text/calendar; charset=UTF-8; method=REQUEST") {
		t.Fatalf("queued invitation is not signed iMIP: %v %s", err, rawOutbox)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, "text/calendar", strings.Replace(create, "SUMMARY:Meeting", "SUMMARY:Changed", 1), http.Header{"If-Match": {`"wrong-etag"`}}), http.StatusPreconditionFailed)
	assertMessages(1, "REQUEST")
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, "text/calendar", create, nil), http.StatusCreated)
	assertMessages(1, "REQUEST")
	mailBox := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_box", map[string]interface{}{
		"name": "INBOX", "subscribed": true, "uidvalidity": 1, "nextuid": 2,
		"attributes": "", "flags": "\\Recent", "permanent_flags": "\\Seen", "mail_account_id": mailAccount,
	})
	reply := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REPLY\r\nPRODID:-//Guest//EN\r\nBEGIN:VEVENT\r\nUID:itip-outbound-e2e\r\nDTSTAMP:20261008T010000Z\r\nSEQUENCE:0\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=ACCEPTED:mailto:guest@example.test\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	raw := "From: guest@example.test\r\nTo: organizer@localhost\r\nSubject: Re: Meeting\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=itip-test\r\n\r\n--itip-test\r\nContent-Type: text/calendar; method=REPLY\r\nContent-Disposition: attachment; filename=reply.ics\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte(reply)) + "\r\n--itip-test--\r\n"
	mailAttrs := map[string]interface{}{
		"message_id": "itip-reply@example.test", "mail_id": "itip-reply", "from_address": "guest@example.test",
		"internal_date": time.Now().UTC().Format(time.RFC3339), "to_address": "organizer@localhost",
		"reply_to_address": "guest@example.test", "sender_address": "guest@example.test", "subject": "Re: Meeting",
		"body": "", "mail": base64.StdEncoding.EncodeToString([]byte(raw)), "spam_score": 0,
		"hash": "itip-reply", "content_type": "multipart/mixed", "recipient": "organizer@localhost",
		"has_attachment": true, "ip_addr": "127.0.0.1", "return_path": "guest@example.test",
		"is_tls": false, "seen": false, "recent": true, "deleted": false, "uid": 1,
		"spam": false, "size": len(raw), "flags": "\\Recent", "mail_box_id": mailBox,
	}
	spoofAttrs := make(map[string]interface{}, len(mailAttrs))
	for key, value := range mailAttrs {
		spoofAttrs[key] = value
	}
	spoofAttrs["message_id"] = "itip-spoof@example.test"
	spoofAttrs["mail_id"] = "itip-spoof"
	spoofAttrs["hash"] = "itip-spoof"
	spoofAttrs["from_address"] = "intruder@example.test"
	spoofMailID := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail", spoofAttrs)
	processURL := base + "/action/mail/process_itip"
	spoofBody := fmt.Sprintf(`{"attributes":{"mail_id":"%s"}}`, spoofMailID)
	spoofResponse := davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", spoofBody, nil)
	if spoofResponse.err != nil {
		t.Fatal(spoofResponse.err)
	}
	if spoofResponse.status == http.StatusOK && strings.Contains(spoofResponse.body, `"events":1`) {
		t.Fatalf("spoofed reply was applied: %s", spoofResponse.body)
	}
	assertMessages(1, "REQUEST")
	beforeReply := davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(beforeReply.body, "PARTSTAT=NEEDS-ACTION") {
		t.Fatalf("spoofed reply changed event: %s", beforeReply.body)
	}
	mailID := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail", mailAttrs)
	processBody := fmt.Sprintf(`{"attributes":{"mail_id":"%s"}}`, mailID)
	denied := davE2ERequest(client, http.MethodPost, processURL, token, "application/json", processBody, nil)
	if denied.err != nil {
		t.Fatal(denied.err)
	}
	if denied.status == http.StatusOK && strings.Contains(denied.body, `"events":1`) {
		t.Fatalf("ordinary user processed administrator action: %s", denied.body)
	}
	processed := davE2EExpect(t, davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", processBody, nil), http.StatusOK)
	inboxPath := base + "/caldav/" + owner[1] + "/schedule-inbox/"
	inbox := davE2EExpect(t, davE2ERequest(client, "PROPFIND", inboxPath, token, "application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/><D:getetag/></D:prop></D:propfind>`, http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	if !strings.Contains(inbox.body, "/schedule-inbox/") || !strings.Contains(inbox.body, "schedule-inbox") || !strings.Contains(inbox.body, ".ics") {
		t.Fatalf("processed reply is absent from recipient inbox: %s", inbox.body)
	}
	inboxObject := regexp.MustCompile(`/caldav/` + owner[1] + `/schedule-inbox/[0-9a-f-]{36}\.ics`).FindString(inbox.body)
	if inboxObject == "" {
		t.Fatalf("inbox did not list a message resource: %s", inbox.body)
	}
	inboxMessage := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+inboxObject, token, "", "", nil), http.StatusOK)
	if !strings.Contains(inboxMessage.body, "METHOD:REPLY") || strings.Contains(inboxMessage.body, "From: guest@example.test") {
		t.Fatalf("inbox exposed wrong content: %s", inboxMessage.body)
	}
	multiget := `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><C:calendar-data/></D:prop><D:href>` + inboxObject + `</D:href></C:calendar-multiget>`
	inboxReport := davE2EExpect(t, davE2ERequest(client, "REPORT", inboxPath, token, "application/xml", multiget, nil), http.StatusMultiStatus)
	if !strings.Contains(inboxReport.body, "METHOD:REPLY") || !strings.Contains(inboxReport.body, inboxObject) {
		t.Fatalf("inbox multiget failed: %s", inboxReport.body)
	}
	inboxQuery := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"><C:time-range start="20300101T000000Z" end="20300102T000000Z"/></C:comp-filter></C:comp-filter></C:filter></C:calendar-query>`
	queryReport := davE2EExpect(t, davE2ERequest(client, "REPORT", inboxPath, token, "application/xml", inboxQuery, nil), http.StatusMultiStatus)
	if !strings.Contains(queryReport.body, inboxObject) {
		t.Fatalf("inbox query omitted an iTIP reply without DTSTART: %s", queryReport.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+inboxObject, adminToken, "", "", nil), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+inboxObject, token, "", "", nil), http.StatusNoContent)
	missingReport := davE2EExpect(t, davE2ERequest(client, "REPORT", inboxPath, token, "application/xml", multiget, nil), http.StatusMultiStatus)
	if !strings.Contains(missingReport.body, inboxObject) || !strings.Contains(missingReport.body, "<D:status>HTTP/1.1 404 Not Found</D:status>") {
		t.Fatalf("acknowledged multiget item needs a per-resource 404: %s", missingReport.body)
	}
	afterAcknowledge := davE2EExpect(t, davE2ERequest(client, "PROPFIND", inboxPath, token, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`, http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	if strings.Contains(afterAcknowledge.body, inboxObject) {
		t.Fatalf("acknowledged message remains in inbox: %s", afterAcknowledge.body)
	}
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", inboxPath, adminToken, "application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`, nil), http.StatusForbidden)
	read := davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(read.body, "PARTSTAT=ACCEPTED") || !strings.Contains(read.body, "SCHEDULE-STATUS=2.0") {
		messages := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)
		t.Fatalf("reply did not update attendee status: %s; action: %s; messages: %#v", read.body, processed.body, messages)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", processBody, nil), http.StatusOK)
	assertMessages(2, "REPLY")
	duplicateAttrs := make(map[string]interface{}, len(mailAttrs))
	for key, value := range mailAttrs {
		duplicateAttrs[key] = value
	}
	duplicateAttrs["message_id"] = "itip-reply-redelivery@example.test"
	duplicateAttrs["mail_id"] = "itip-reply-redelivery"
	duplicateAttrs["hash"] = "itip-reply-redelivery"
	duplicateAttrs["uid"] = 2
	duplicateID := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail", duplicateAttrs)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json",
		fmt.Sprintf(`{"attributes":{"mail_id":"%s"}}`, duplicateID), nil), http.StatusOK)
	assertMessages(2, "REPLY")
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, objectURL, token, "", "", nil), http.StatusNoContent)
	assertMessages(3, "CANCEL")
	cancelMessages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		base+"/api/cal_mail?page%5Bsize%5D=100", token, nil, http.StatusOK))
	for _, item := range cancelMessages {
		message := item.(map[string]interface{})
		if message["attributes"].(map[string]interface{})["method"] != "CANCEL" {
			continue
		}
		cancelAttrs := message["attributes"].(map[string]interface{})
		if cancelAttrs["sequence"] != float64(1) || !strings.Contains(fmt.Sprint(cancelAttrs["icalendar"]), "STATUS:CANCELLED") {
			t.Fatalf("event cancellation lacks iTIP sequence or status: %#v", cancelAttrs)
		}
		cancelStatusBody := fmt.Sprintf(`{"attributes":{"collection_id":"%s","message_id":"%s"}}`, collectionID, message["id"])
		davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/scheduling_status",
			token, "application/json", cancelStatusBody, nil), http.StatusOK)
		break
	}
	baseEvent := "BEGIN:VEVENT\r\nUID:itip-recurring-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261012T120000Z\r\nDTEND:20261012T130000Z\r\nRRULE:FREQ=DAILY;COUNT=2\r\nSEQUENCE:0\r\nSUMMARY:Series\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:guest@example.test\r\nEND:VEVENT\r\n"
	override := "BEGIN:VEVENT\r\nUID:itip-recurring-e2e\r\nRECURRENCE-ID:20261013T120000Z\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261013T140000Z\r\nDTEND:20261013T150000Z\r\nSEQUENCE:0\r\nSUMMARY:Changed occurrence\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:guest@example.test\r\nEND:VEVENT\r\n"
	wrap := func(events string) string {
		return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\n" + events + "END:VCALENDAR\r\n"
	}
	recurringURL := base + calendarPath + "recurring.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, recurringURL, token, "text/calendar", wrap(baseEvent+override), nil), http.StatusCreated)
	recurringMessages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(recurringMessages) != 5 {
		t.Fatalf("recurring create queued %d total messages, want 5", len(recurringMessages))
	}
	changedOverride := strings.Replace(override, "SUMMARY:Changed occurrence", "SUMMARY:Rescheduled occurrence", 1)
	changedOverride = strings.Replace(changedOverride, "SEQUENCE:0", "SEQUENCE:1", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, recurringURL, token, "text/calendar", wrap(baseEvent+changedOverride), nil), http.StatusCreated)
	recurringMessages = accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(recurringMessages) != 6 {
		t.Fatalf("occurrence update queued %d total messages, want 6", len(recurringMessages))
	}
	var occurrenceRequest bool
	for _, item := range recurringMessages {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "itip-recurring-e2e" && attrs["method"] == "REQUEST" && attrs["sequence"] == float64(1) {
			body := fmt.Sprint(attrs["icalendar"])
			occurrenceRequest = attrs["recurrence_id"] == "20261013T120000Z" && strings.Count(body, "BEGIN:VEVENT") == 1
		}
	}
	if !occurrenceRequest {
		t.Fatalf("rescheduled occurrence did not get one-instance REQUEST: %#v", recurringMessages)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, recurringURL, token, "text/calendar", wrap(baseEvent), nil), http.StatusCreated)
	recurringMessages = accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(recurringMessages) != 7 {
		t.Fatalf("occurrence removal queued %d total messages, want 7", len(recurringMessages))
	}
	var occurrenceCancel bool
	for _, item := range recurringMessages {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "itip-recurring-e2e" && attrs["method"] == "CANCEL" && attrs["recurrence_id"] == "20261013T120000Z" {
			body := fmt.Sprint(attrs["icalendar"])
			occurrenceCancel = strings.Count(body, "BEGIN:VEVENT") == 1 &&
				strings.Contains(body, "STATUS:CANCELLED") && attrs["sequence"] == float64(2)
		}
	}
	if !occurrenceCancel {
		t.Fatalf("removed occurrence did not get one-instance CANCEL: %#v", recurringMessages)
	}
	unconnectedID := davE2ECollectionReferenceID(t, collection, "unconnected")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+unconnectedID+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", relation, nil), http.StatusNoContent)
	legacyEvent := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+unconnectedPath+"meeting.ics", token, "", "", nil), http.StatusOK)
	if legacyEvent.header.Get("Schedule-Tag") == "" {
		t.Fatal("event created before connecting its calendar has no schedule tag")
	}
	sharedWithinPrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+owner[1]+"/", token, "application/xml", scheduleProperties, nil), http.StatusMultiStatus)
	if strings.Count(sharedWithinPrincipal.body, "mailto:organizer@localhost") != 1 {
		t.Fatalf("one account linked to two calendars produced duplicate addresses: %s", sharedWithinPrincipal.body)
	}
	autoOptions := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, base+calendarPath, token, "", "", http.Header{"Origin": {"http://localhost"}}), http.StatusNoContent)
	if strings.Contains(autoOptions.header.Get("DAV"), "calendar-auto-schedule") {
		t.Fatalf("ambiguous incoming calendar advertised automatic scheduling: %s", autoOptions.header.Get("DAV"))
	}
	pendingRequest := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nPRODID:-//Guest//EN\r\nBEGIN:VEVENT\r\nUID:itip-pending-e2e\r\nDTSTAMP:20261008T010000Z\r\nDTSTART:20261021T120000Z\r\nDTEND:20261021T130000Z\r\nSEQUENCE:0\r\nSUMMARY:Pending invitation\r\nORGANIZER;SCHEDULE-STATUS=1.2:mailto:guest@example.test\r\nATTENDEE;SCHEDULE-AGENT=CLIENT;SCHEDULE-STATUS=1.2:mailto:organizer@localhost\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	pendingRaw := "From: guest@example.test\r\nTo: organizer@localhost\r\nSubject: Pending invitation\r\nMIME-Version: 1.0\r\nContent-Type: text/calendar; method=REQUEST\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte(pendingRequest)) + "\r\n"
	pendingAttrs := make(map[string]interface{}, len(mailAttrs))
	for key, value := range mailAttrs {
		pendingAttrs[key] = value
	}
	pendingAttrs["message_id"], pendingAttrs["mail_id"], pendingAttrs["hash"] = "itip-pending@example.test", "itip-pending", "itip-pending"
	pendingAttrs["mail"], pendingAttrs["size"], pendingAttrs["uid"] = base64.StdEncoding.EncodeToString([]byte(pendingRaw)), len(pendingRaw), 4
	pendingMailID := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail", pendingAttrs)
	pendingBody := fmt.Sprintf(`{"attributes":{"mail_id":"%s"}}`, pendingMailID)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", pendingBody, nil), http.StatusOK)
	pendingCalendar := "/caldav/" + owner[1] + "/calendars/unconnected/"
	pendingList := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+pendingCalendar, token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	pendingGeneratedPath := regexp.MustCompile(regexp.QuoteMeta(pendingCalendar) + `[0-9a-f-]{36}\.ics`)
	if pendingGeneratedPath.MatchString(pendingList.body) {
		t.Fatalf("ambiguous calendar selection wrote an event: %s", pendingList.body)
	}
	selectedPending := fmt.Sprintf(`{"attributes":{"mail_id":"%s","collection_id":"%s"}}`, pendingMailID, unconnectedID)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", selectedPending, nil), http.StatusOK)
	appliedList := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+pendingCalendar, token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	if !pendingGeneratedPath.MatchString(appliedList.body) {
		t.Fatalf("reprocessing pending invitation did not create event: %s", appliedList.body)
	}
	pendingStored := davE2EExpect(t, davE2ERequest(client, http.MethodGet,
		base+pendingGeneratedPath.FindString(appliedList.body), token, "", "", nil), http.StatusOK)
	if strings.Contains(pendingStored.body, "SCHEDULE-STATUS") || strings.Contains(pendingStored.body, "SCHEDULE-AGENT") {
		t.Fatalf("incoming mail planted scheduling state in attendee calendar: %s", pendingStored.body)
	}
	secondToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "itip-second-owner")
	secondPrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", secondToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	secondOwner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(secondPrincipal.body)
	if len(secondOwner) != 2 {
		t.Fatalf("missing second principal: %s", secondPrincipal.body)
	}
	secondPath := "/caldav/" + secondOwner[1] + "/calendars/second/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+secondPath, secondToken, "", "", nil), http.StatusCreated)
	secondCollections := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/collection?page%5Bsize%5D=100", secondToken, nil, http.StatusOK)
	secondCollectionID := davE2ECollectionReferenceID(t, secondCollections, "second")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/collection/"+secondCollectionID+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", relation, nil), http.StatusNoContent)
	ambiguous := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+owner[1]+"/", token, "application/xml", scheduleProperties, nil), http.StatusMultiStatus)
	if strings.Contains(ambiguous.body, "mailto:organizer@localhost") || !strings.Contains(ambiguous.body, "404 Not Found") {
		t.Fatalf("shared address still advertises ambiguous principal: %s", ambiguous.body)
	}
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+secondOwner[1]+"/", secondToken, "application/xml", scheduleProperties, nil), http.StatusMultiStatus)
	sharedRequest := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nPRODID:-//Guest//EN\r\nBEGIN:VEVENT\r\nUID:itip-shared-inbound-e2e\r\nDTSTAMP:20261008T010000Z\r\nDTSTART:20261020T120000Z\r\nDTEND:20261020T130000Z\r\nSEQUENCE:0\r\nSUMMARY:Shared sender invitation\r\nORGANIZER:mailto:guest@example.test\r\nATTENDEE:mailto:organizer@localhost\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	sharedRaw := "From: guest@example.test\r\nTo: organizer@localhost\r\nSubject: Shared sender invitation\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=itip-shared\r\n\r\n--itip-shared\r\nContent-Type: text/calendar; method=REQUEST\r\nContent-Disposition: attachment; filename=invite.ics\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte(sharedRequest)) + "\r\n--itip-shared--\r\n"
	sharedAttrs := make(map[string]interface{}, len(mailAttrs))
	for key, value := range mailAttrs {
		sharedAttrs[key] = value
	}
	sharedAttrs["message_id"], sharedAttrs["mail_id"], sharedAttrs["hash"] = "itip-shared@example.test", "itip-shared", "itip-shared"
	sharedAttrs["mail"], sharedAttrs["size"], sharedAttrs["uid"] = base64.StdEncoding.EncodeToString([]byte(sharedRaw)), len(sharedRaw), 3
	sharedMailID := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail", sharedAttrs)
	sharedBody := fmt.Sprintf(`{"attributes":{"mail_id":"%s"}}`, sharedMailID)
	withoutRoute := davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", sharedBody, nil)
	if withoutRoute.err != nil || withoutRoute.status == http.StatusOK && strings.Contains(withoutRoute.body, `"events":1`) {
		t.Fatalf("ambiguous invitation was accepted without an explicit calendar: %+v", withoutRoute)
	}
	wrongRoute := fmt.Sprintf(`{"attributes":{"mail_id":"%s","collection_id":"%s"}}`, sharedMailID, "11111111-1111-1111-1111-111111111111")
	wrongResult := davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", wrongRoute, nil)
	if wrongResult.err != nil || wrongResult.status == http.StatusOK && strings.Contains(wrongResult.body, `"events":1`) {
		t.Fatalf("unconnected calendar was accepted as invitation route: %+v", wrongResult)
	}
	selectedRoute := fmt.Sprintf(`{"attributes":{"mail_id":"%s","collection_id":"%s"}}`, sharedMailID, secondCollectionID)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, processURL, adminToken, "application/json", selectedRoute, nil), http.StatusOK)
	sharedInbox := base + "/caldav/" + secondOwner[1] + "/schedule-inbox/"
	sharedInboxResult := davE2EExpect(t, davE2ERequest(client, "PROPFIND", sharedInbox, secondToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`, http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	if !strings.Contains(sharedInboxResult.body, ".ics") {
		t.Fatalf("routed invitation is absent from selected owner's inbox: %s", sharedInboxResult.body)
	}
	secondEvents := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+secondPath, secondToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	secondEventPath := regexp.MustCompile(regexp.QuoteMeta(secondPath) + `[0-9a-f-]{36}\.ics`).FindString(secondEvents.body)
	if secondEventPath == "" {
		t.Fatalf("explicitly routed invitation did not create a calendar event: %s", secondEvents.body)
	}
	secondEvent := davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+secondEventPath, secondToken, "", "", nil), http.StatusOK)
	if !strings.Contains(secondEvent.body, "UID:itip-shared-inbound-e2e") || strings.Contains(secondEvent.body, "METHOD:REQUEST") {
		t.Fatalf("routed invitation created invalid event content: %s", secondEvent.body)
	}
	foreignEvent := davE2ERequest(client, http.MethodGet, base+secondEventPath, token, "", "", nil)
	if foreignEvent.err != nil || foreignEvent.status == http.StatusOK {
		t.Fatalf("another principal read the routed event: %+v", foreignEvent)
	}
	otherInboxResult := davE2ERequest(client, "PROPFIND", sharedInbox, token, "application/xml", scheduleProperties, nil)
	if otherInboxResult.err != nil || otherInboxResult.status != http.StatusForbidden {
		t.Fatalf("another principal reached selected owner's inbox: %+v", otherInboxResult)
	}

	adminPrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", adminToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	adminOwner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(adminPrincipal.body)
	if len(adminOwner) != 2 {
		t.Fatalf("missing administrator principal: %s", adminPrincipal.body)
	}
	exchangeRef := accessGroupsE2ECreateRecord(t, client, base, adminToken, "data_exchange", map[string]interface{}{
		"name": "process calendar mail", "source_type": "self", "source_attributes": `{"name":"mail"}`,
		"target_type": "action", "target_attributes": `{"type":"mail","action":"process_itip","attributes":{}}`,
		"attributes": `{"name":"mail","hook":"after","methods":["post"]}`, "options": `{}`,
	})
	exchangeUser := fmt.Sprintf(`{"data":{"type":"user_account","id":"%s"}}`, adminOwner[1])
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		base+"/api/data_exchange/"+exchangeRef+"/relationships/as_user_id", adminToken,
		"application/vnd.api+json", exchangeUser, nil), http.StatusNoContent)
	process.stopProcess()
	activeBase, activeProcess := start()
	defer activeProcess.stopProcess()
	autoReply := strings.ReplaceAll(reply, "itip-outbound-e2e", "itip-recurring-e2e")
	autoReply = strings.ReplaceAll(autoReply, "PARTSTAT=ACCEPTED", "PARTSTAT=DECLINED")
	autoRaw := strings.Replace(raw, base64.StdEncoding.EncodeToString([]byte(reply)),
		base64.StdEncoding.EncodeToString([]byte(autoReply)), 1)
	var smtpClient *smtp.Client
	for attempt := 0; attempt < 30; attempt++ {
		smtpClient, err = smtp.Dial(fmt.Sprintf("127.0.0.1:%d", smtpPort))
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("SMTP receiver did not start: %v", err)
	}
	if err := smtpClient.Mail("guest@example.test"); err != nil {
		t.Fatal(err)
	}
	if err := smtpClient.Rcpt("organizer@localhost"); err != nil {
		t.Fatal(err)
	}
	smtpData, err := smtpClient.Data()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := smtpData.Write([]byte(autoRaw)); err != nil {
		t.Fatal(err)
	}
	if err := smtpData.Close(); err != nil {
		t.Fatal(err)
	}
	if err := smtpClient.Quit(); err != nil {
		t.Fatal(err)
	}
	processExchangeURL := activeBase + "/action/exchange_run/process_data_exchange_executions"
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, processExchangeURL, adminToken,
		"application/json", `{"attributes":{}}`, nil), http.StatusOK)
	autoApplied := davE2EExpect(t, davE2ERequest(client, http.MethodGet,
		activeBase+calendarPath+"recurring.ics", token, "", "", nil), http.StatusOK)
	if !strings.Contains(autoApplied.body, "PARTSTAT=DECLINED") {
		t.Fatalf("mail data exchange did not apply a reply for a shared sending account: %s", autoApplied.body)
	}

	attendeeToken := accessGroupsE2ESignupSigninUser(t, client, activeBase, adminToken, "itip-attendee")
	attendeePrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", activeBase+"/caldav/", attendeeToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	attendeeOwner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(attendeePrincipal.body)
	if len(attendeeOwner) != 2 {
		t.Fatalf("missing attendee principal: %s", attendeePrincipal.body)
	}
	attendeeAccount := accessGroupsE2ECreateRecord(t, client, activeBase, adminToken, "mail_account", map[string]interface{}{
		"username": "attendee@localhost", "password": "testpass123", "password_md5": "testpass123", "mail_server_id": mailServer,
	})
	attendeePath := "/caldav/" + attendeeOwner[1] + "/calendars/meetings/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", activeBase+attendeePath, attendeeToken, "", "", nil), http.StatusCreated)
	attendeeCollections := accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/collection?page%5Bsize%5D=100", attendeeToken, nil, http.StatusOK)
	attendeeCollection := davE2ECollectionReferenceID(t, attendeeCollections, "meetings")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
		activeBase+"/api/collection/"+attendeeCollection+"/relationships/scheduling_mail_account_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, attendeeAccount), nil), http.StatusNoContent)
	meeting := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:itip-two-users-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261016T120000Z\r\nDTEND:20261016T130000Z\r\nSEQUENCE:0\r\nSUMMARY:Two user meeting\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@localhost\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	organizerURL := activeBase + calendarPath + "two-users.ics"
	findAttendeeEvent := func() string {
		t.Helper()
		listing := davE2EExpect(t, davE2ERequest(client, "PROPFIND", activeBase+attendeePath, attendeeToken,
			"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`,
			http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
		objectPath := regexp.MustCompile(regexp.QuoteMeta(attendeePath) + `[0-9a-f-]{36}\.ics`).FindString(listing.body)
		if objectPath == "" {
			t.Fatalf("attendee calendar has no scheduled event: %s", listing.body)
		}
		return activeBase + objectPath
	}
	deliveredOutbox := make(map[string]bool)
	sendLatest := func(recipient string) {
		t.Helper()
		rows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
			activeBase+"/api/outbox?included_relations=mail&page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
		var selectedID string
		var selectedBody string
		for _, item := range rows {
			row := item.(map[string]interface{})
			attrs := row["attributes"].(map[string]interface{})
			if attrs["to_address"] != recipient || deliveredOutbox[row["id"].(string)] {
				continue
			}
			candidateID := row["id"].(string)
			if candidateID > selectedID {
				selectedID = candidateID
				selectedBody, _ = attrs["mail"].(string)
			}
		}
		if selectedBody == "" {
			t.Fatalf("no queued mail for %s", recipient)
		}
		deliveredOutbox[selectedID] = true
		if smtpPort == 25 {
			davE2EExpect(t, davE2ERequest(client, http.MethodPost, activeBase+"/action/outbox/process_outbox",
				adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
			processed := accessGroupsE2ERequestJSON(t, client, http.MethodGet, activeBase+"/api/outbox/"+selectedID,
				adminToken, nil, http.StatusOK)
			outboxData := processed.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
			if outboxData["sent"] != true && outboxData["sent"] != float64(1) {
				t.Fatalf("outbox worker did not deliver %s to local SMTP: sent=%v error=%v retries=%v",
					recipient, outboxData["sent"], outboxData["last_error"], outboxData["retry_count"])
			}
			davE2EExpect(t, davE2ERequest(client, http.MethodPost, activeBase+"/action/cal_mail/reconcile_delivery",
				adminToken, "application/json", `{"attributes":{}}`, nil), http.StatusOK)
			if recipient == "attendee@localhost" {
				sentEvent := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
				if !strings.Contains(sentEvent.body, "SCHEDULE-STATUS=1.1") {
					t.Fatalf("SMTP acceptance was not reflected in organizer event: %s", sentEvent.body)
				}
			}
			for attempt := 0; attempt < 4; attempt++ {
				davE2EExpect(t, davE2ERequest(client, http.MethodPost, processExchangeURL, adminToken,
					"application/json", `{"attributes":{}}`, nil), http.StatusOK)
			}
			return
		}
		wire, err := base64.StdEncoding.DecodeString(selectedBody)
		if err != nil {
			t.Fatal(err)
		}
		smtpClient, err := smtp.Dial(fmt.Sprintf("127.0.0.1:%d", smtpPort))
		if err != nil {
			t.Fatal(err)
		}
		if err := smtpClient.Mail("relay@example.test"); err != nil {
			t.Fatal(err)
		}
		if err := smtpClient.Rcpt(recipient); err != nil {
			t.Fatal(err)
		}
		writer, err := smtpClient.Data()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(wire); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := smtpClient.Quit(); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 4; attempt++ {
			davE2EExpect(t, davE2ERequest(client, http.MethodPost, processExchangeURL, adminToken,
				"application/json", `{"attributes":{}}`, nil), http.StatusOK)
		}
	}
	deliverCalendarMail := func(from, to, method, body string) {
		t.Helper()
		raw := "From: " + from + "\r\nTo: " + to + "\r\nSubject: Calendar update\r\nMIME-Version: 1.0\r\nContent-Type: text/calendar; method=" + method + "\r\nContent-Transfer-Encoding: base64\r\n\r\n" + base64.StdEncoding.EncodeToString([]byte(body)) + "\r\n"
		mailClient, err := smtp.Dial(fmt.Sprintf("127.0.0.1:%d", smtpPort))
		if err != nil {
			t.Fatal(err)
		}
		if err := mailClient.Mail(from); err != nil {
			t.Fatal(err)
		}
		if err := mailClient.Rcpt(to); err != nil {
			t.Fatal(err)
		}
		writer, err := mailClient.Data()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(raw)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := mailClient.Quit(); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 4; attempt++ {
			davE2EExpect(t, davE2ERequest(client, http.MethodPost, processExchangeURL, adminToken,
				"application/json", `{"attributes":{}}`, nil), http.StatusOK)
		}
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, organizerURL, token, "text/calendar", meeting, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, activeBase+calendarPath+"duplicate-uid.ics", token,
		"text/calendar", meeting, nil), http.StatusConflict)
	sendLatest("attendee@localhost")
	attendeeInbox := activeBase + "/caldav/" + attendeeOwner[1] + "/schedule-inbox/"
	requestInbox := davE2EExpect(t, davE2ERequest(client, "PROPFIND", attendeeInbox, attendeeToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	if !strings.Contains(requestInbox.body, ".ics") {
		t.Fatalf("attendee did not receive request: %s", requestInbox.body)
	}
	attendeeURL := findAttendeeEvent()
	createdAttendee := davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeURL, attendeeToken, "", "", nil), http.StatusOK)
	initialAttendeeTag := createdAttendee.header.Get("Schedule-Tag")
	if initialAttendeeTag == "" {
		t.Fatal("scheduled attendee event has no Schedule-Tag")
	}
	tagProperty := davE2EExpect(t, davE2ERequest(client, "PROPFIND", attendeeURL, attendeeToken,
		"application/xml", `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:schedule-tag/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(tagProperty.body, "schedule-tag") || !strings.Contains(tagProperty.body, strings.Trim(initialAttendeeTag, `"`)) {
		t.Fatalf("scheduled attendee event has no matching schedule-tag property: %s", tagProperty.body)
	}
	if !strings.Contains(createdAttendee.body, "UID:itip-two-users-e2e") || !strings.Contains(createdAttendee.body, "PARTSTAT=NEEDS-ACTION") {
		t.Fatalf("incoming invitation did not create attendee event: %s", createdAttendee.body)
	}
	forgedSummary := strings.Replace(createdAttendee.body, "SUMMARY:Two user meeting", "SUMMARY:Forged by attendee", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeURL, attendeeToken,
		"text/calendar", forgedSummary, http.Header{"If-Schedule-Tag-Match": {initialAttendeeTag}}), http.StatusForbidden)
	forgedEnvelope := strings.Replace(createdAttendee.body, "END:VCALENDAR", "X-OWNER:forged\r\nEND:VCALENDAR", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeURL, attendeeToken,
		"text/calendar", forgedEnvelope, http.Header{"If-Schedule-Tag-Match": {initialAttendeeTag}}), http.StatusForbidden)
	autoEventID := accessGroupsE2EFindResourceID(t, client, activeBase, adminToken, "calendar", "rpath",
		strings.TrimPrefix(attendeeURL, activeBase))
	autoRecord := accessGroupsE2ERequestJSON(t, client, http.MethodGet, activeBase+"/api/calendar/"+autoEventID,
		adminToken, nil, http.StatusOK)
	autoAttrs := autoRecord.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if autoAttrs["user_account_id"] != attendeeOwner[1] {
		t.Fatalf("scheduled event owner = %v, want attendee %s", autoAttrs["user_account_id"], attendeeOwner[1])
	}
	repeatedRequest := strings.Replace(meeting, "VERSION:2.0\r\n", "VERSION:2.0\r\nMETHOD:REQUEST\r\n", 1)
	deliverCalendarMail("organizer@localhost", "attendee@localhost", "REQUEST", repeatedRequest)
	repeatedListing := davE2EExpect(t, davE2ERequest(client, "PROPFIND", activeBase+attendeePath, attendeeToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:getetag/></D:prop></D:propfind>`,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	repeatedPaths := regexp.MustCompile(regexp.QuoteMeta(attendeePath)+`[0-9a-f-]{36}\.ics`).FindAllString(repeatedListing.body, -1)
	if len(repeatedPaths) != 1 || activeBase+repeatedPaths[0] != attendeeURL {
		t.Fatalf("duplicate invitation changed attendee event count or path: %v, original %s", repeatedPaths, attendeeURL)
	}
	accepted := strings.Replace(createdAttendee.body, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	organizerBeforeReply := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	organizerTag := organizerBeforeReply.header.Get("Schedule-Tag")
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeURL, attendeeToken, "text/calendar", accepted, nil), http.StatusCreated)
	sendLatest("organizer@localhost")
	organizerAccepted := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	if organizerTag == "" || organizerAccepted.header.Get("Schedule-Tag") != organizerTag || organizerAccepted.header.Get("ETag") == organizerBeforeReply.header.Get("ETag") {
		t.Fatalf("RSVP must change organizer ETag but retain schedule tag: before=%q/%q after=%q/%q", organizerBeforeReply.header.Get("ETag"), organizerTag, organizerAccepted.header.Get("ETag"), organizerAccepted.header.Get("Schedule-Tag"))
	}
	if !strings.Contains(organizerAccepted.body, "PARTSTAT=ACCEPTED") || !strings.Contains(organizerAccepted.body, "SCHEDULE-STATUS=2.0") {
		t.Fatalf("organizer did not receive acceptance: %s", organizerAccepted.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, organizerURL, token, "text/calendar",
		organizerBeforeReply.body, http.Header{"If-Schedule-Tag-Match": {organizerTag}}), http.StatusCreated)
	mergedOrganizer := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(mergedOrganizer.body, "PARTSTAT=ACCEPTED") || mergedOrganizer.header.Get("Schedule-Tag") == organizerTag {
		t.Fatalf("schedule-tag PUT did not retain server RSVP and advance tag: %s", mergedOrganizer.body)
	}
	declined := strings.Replace(accepted, "PARTSTAT=ACCEPTED", "PARTSTAT=DECLINED", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeURL, attendeeToken, "text/calendar", declined, nil), http.StatusCreated)
	sendLatest("organizer@localhost")
	organizerDeclined := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(organizerDeclined.body, "PARTSTAT=DECLINED") {
		t.Fatalf("organizer did not receive decline: %s", organizerDeclined.body)
	}
	rescheduled := strings.Replace(meeting, "SEQUENCE:0", "SEQUENCE:1", 1)
	rescheduled = strings.Replace(rescheduled, "20261016T120000Z", "20261016T140000Z", 1)
	rescheduled = strings.Replace(rescheduled, "20261016T130000Z", "20261016T150000Z", 1)
	rescheduled = strings.Replace(rescheduled, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, organizerURL, token, "text/calendar", rescheduled, nil), http.StatusCreated)
	organizerAfterReschedule := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(organizerAfterReschedule.body, "PARTSTAT=NEEDS-ACTION") {
		t.Fatalf("rescheduling retained an earlier RSVP: %s", organizerAfterReschedule.body)
	}
	sendLatest("attendee@localhost")
	autoRescheduled := davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeURL, attendeeToken, "", "", nil), http.StatusOK)
	if autoRescheduled.header.Get("Schedule-Tag") == initialAttendeeTag {
		t.Fatal("organizer reschedule did not change attendee schedule tag")
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeURL, attendeeToken, "text/calendar", accepted,
		http.Header{"If-Schedule-Tag-Match": {initialAttendeeTag}}), http.StatusPreconditionFailed)
	if !strings.Contains(autoRescheduled.body, "20261016T140000Z") || !strings.Contains(autoRescheduled.body, "PARTSTAT=NEEDS-ACTION") {
		t.Fatalf("reschedule was not applied to attendee event: %s", autoRescheduled.body)
	}
	staleRequest := strings.Replace(meeting, "DTSTAMP:20261008T000000Z", "DTSTAMP:20261008T020000Z", 1)
	deliverCalendarMail("organizer@localhost", "attendee@localhost", "REQUEST", staleRequest)
	stillRescheduled := davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeURL, attendeeToken, "", "", nil), http.StatusOK)
	if !strings.Contains(stillRescheduled.body, "20261016T140000Z") || strings.Contains(stillRescheduled.body, "20261016T120000Z") {
		t.Fatalf("stale request replaced attendee reschedule: %s", stillRescheduled.body)
	}
	attendeeRescheduled := strings.Replace(autoRescheduled.body, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeURL, attendeeToken, "text/calendar", attendeeRescheduled, nil), http.StatusCreated)
	sendLatest("organizer@localhost")
	organizerRescheduled := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(organizerRescheduled.body, "PARTSTAT=ACCEPTED") || !strings.Contains(organizerRescheduled.body, "SEQUENCE:1") {
		t.Fatalf("rescheduled response did not arrive: %s", organizerRescheduled.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, attendeeURL, attendeeToken, "", "", nil), http.StatusNoContent)
	sendLatest("organizer@localhost")
	organizerDeclineOnDelete := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(organizerDeclineOnDelete.body, "PARTSTAT=DECLINED") {
		t.Fatalf("attendee deletion did not send a declined reply: %s", organizerDeclineOnDelete.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, organizerURL, token, "", "", nil), http.StatusNoContent)
	sendLatest("attendee@localhost")
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeURL, attendeeToken, "", "", nil), http.StatusNotFound)
	seriesMaster := strings.ReplaceAll(baseEvent, "itip-recurring-e2e", "itip-two-users-series")
	seriesMaster = strings.ReplaceAll(seriesMaster, "guest@example.test", "attendee@localhost")
	seriesOverride := strings.ReplaceAll(override, "itip-recurring-e2e", "itip-two-users-series")
	seriesOverride = strings.ReplaceAll(seriesOverride, "guest@example.test", "attendee@localhost")
	series := wrap(seriesMaster + seriesOverride)
	organizerSeriesURL := activeBase + calendarPath + "two-users-series.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, organizerSeriesURL, token, "text/calendar", series, nil), http.StatusCreated)
	sendLatest("attendee@localhost")
	sendLatest("attendee@localhost")
	attendeeSeriesURL := findAttendeeEvent()
	autoSeries := davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeSeriesURL, attendeeToken, "", "", nil), http.StatusOK)
	if !strings.Contains(autoSeries.body, "RECURRENCE-ID:20261013T120000Z") {
		t.Fatalf("recurring request did not create attendee series: %s", autoSeries.body)
	}
	lastStatus := strings.LastIndex(autoSeries.body, "PARTSTAT=NEEDS-ACTION")
	if lastStatus < 0 {
		t.Fatalf("recurring attendee event has no pending response: %s", autoSeries.body)
	}
	acceptedOccurrence := autoSeries.body[:lastStatus] + "PARTSTAT=ACCEPTED" + autoSeries.body[lastStatus+len("PARTSTAT=NEEDS-ACTION"):]
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeSeriesURL, attendeeToken, "text/calendar", acceptedOccurrence, nil), http.StatusCreated)
	sendLatest("organizer@localhost")
	organizerSeries := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerSeriesURL, token, "", "", nil), http.StatusOK)
	if !strings.Contains(organizerSeries.body, "RECURRENCE-ID:20261013T120000Z") || !strings.Contains(organizerSeries.body, "PARTSTAT=ACCEPTED") {
		t.Fatalf("recurring occurrence response was not applied: %s", organizerSeries.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, organizerSeriesURL, token, "text/calendar", wrap(seriesMaster), nil), http.StatusCreated)
	sendLatest("attendee@localhost")
	attendeeCancelledOccurrence := davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeSeriesURL, attendeeToken, "", "", nil), http.StatusOK)
	if !strings.Contains(attendeeCancelledOccurrence.body, "RECURRENCE-ID:20261013T120000Z") ||
		!strings.Contains(attendeeCancelledOccurrence.body, "STATUS:CANCELLED") ||
		!strings.Contains(attendeeCancelledOccurrence.body, "RRULE:FREQ=DAILY;COUNT=2") {
		t.Fatalf("occurrence cancellation changed the wrong attendee event: %s", attendeeCancelledOccurrence.body)
	}
	earlyCancel := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:CANCEL\r\nBEGIN:VEVENT\r\nUID:itip-early-cancel-e2e\r\nRECURRENCE-ID:20261024T120000Z\r\nDTSTAMP:20261008T050000Z\r\nDTSTART:20261024T120000Z\r\nDTEND:20261024T130000Z\r\nSEQUENCE:1\r\nSTATUS:CANCELLED\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE:mailto:attendee@localhost\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	deliverCalendarMail("organizer@localhost", "attendee@localhost", "CANCEL", earlyCancel)
	earlyRequest := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nMETHOD:REQUEST\r\nBEGIN:VEVENT\r\nUID:itip-early-cancel-e2e\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261023T120000Z\r\nDTEND:20261023T130000Z\r\nRRULE:FREQ=DAILY;COUNT=2\r\nSEQUENCE:0\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@localhost\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
	deliverCalendarMail("organizer@localhost", "attendee@localhost", "REQUEST", earlyRequest)
	attendeeRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/calendar?page%5Bsize%5D=100", attendeeToken, nil, http.StatusOK))
	var earlyEventPath string
	for _, item := range attendeeRows {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "itip-early-cancel-e2e" {
			earlyEventPath, _ = attrs["rpath"].(string)
		}
	}
	if earlyEventPath == "" {
		t.Fatalf("out-of-order invitation did not create attendee series: %#v", attendeeRows)
	}
	earlyEvent := davE2EExpect(t, davE2ERequest(client, http.MethodGet, activeBase+earlyEventPath, attendeeToken, "", "", nil), http.StatusOK)
	if !strings.Contains(earlyEvent.body, "STATUS:CANCELLED") || !strings.Contains(earlyEvent.body, "RECURRENCE-ID:20261024T120000Z") ||
		!strings.Contains(earlyEvent.body, "RRULE:FREQ=DAILY;COUNT=2") {
		t.Fatalf("pending occurrence cancellation was not applied after request: %s", earlyEvent.body)
	}
	outboxBeforeSuppressedDelete := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/outbox?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, activeBase+earlyEventPath, attendeeToken, "", "",
		http.Header{"Schedule-Reply": {"F"}}), http.StatusNoContent)
	outboxAfterSuppressedDelete := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/outbox?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	if len(outboxAfterSuppressedDelete) != len(outboxBeforeSuppressedDelete) {
		t.Fatalf("Schedule-Reply: F queued an attendee reply: before=%d after=%d", len(outboxBeforeSuppressedDelete), len(outboxAfterSuppressedDelete))
	}
	task := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VTODO\r\nUID:itip-two-users-task\r\nDTSTAMP:20261008T000000Z\r\nDTSTART:20261025T120000Z\r\nDUE:20261026T120000Z\r\nSEQUENCE:0\r\nSUMMARY:Shared task\r\nORGANIZER:mailto:organizer@localhost\r\nATTENDEE;PARTSTAT=NEEDS-ACTION:mailto:attendee@localhost\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	taskURL := activeBase + calendarPath + "two-users-task.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, taskURL, token, "text/calendar", task, nil), http.StatusCreated)
	taskMessages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	queuedTask := false
	for _, item := range taskMessages {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		queuedTask = queuedTask || attrs["uid"] == "itip-two-users-task" && attrs["method"] == "REQUEST" &&
			strings.Contains(fmt.Sprint(attrs["icalendar"]), "BEGIN:VTODO")
	}
	if !queuedTask {
		t.Fatalf("VTODO invitation did not queue an iTIP task: %#v", taskMessages)
	}
	requestTask := strings.Replace(task, "VERSION:2.0\r\n", "VERSION:2.0\r\nMETHOD:REQUEST\r\n", 1)
	deliverCalendarMail("organizer@localhost", "attendee@localhost", "REQUEST", requestTask)
	taskRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/calendar?page%5Bsize%5D=100", attendeeToken, nil, http.StatusOK))
	var attendeeTaskPath string
	for _, item := range taskRows {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "itip-two-users-task" {
			attendeeTaskPath, _ = attrs["rpath"].(string)
		}
	}
	if attendeeTaskPath == "" {
		t.Fatalf("VTODO invitation did not create attendee task: %#v", taskRows)
	}
	attendeeTaskURL := activeBase + attendeeTaskPath
	attendeeTask := davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeTaskURL, attendeeToken, "", "", nil), http.StatusOK)
	if !strings.Contains(attendeeTask.body, "BEGIN:VTODO") || !strings.Contains(attendeeTask.body, "PARTSTAT=NEEDS-ACTION") {
		t.Fatalf("VTODO invitation stored incorrect task: %s", attendeeTask.body)
	}
	acceptedTask := strings.Replace(attendeeTask.body, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, attendeeTaskURL, attendeeToken, "text/calendar", acceptedTask, nil), http.StatusCreated)
	replyTask := strings.Replace(requestTask, "METHOD:REQUEST", "METHOD:REPLY", 1)
	replyTask = strings.Replace(replyTask, "PARTSTAT=NEEDS-ACTION", "PARTSTAT=ACCEPTED", 1)
	replyTask = strings.Replace(replyTask, "END:VTODO", "REQUEST-STATUS:2.0;Success\r\nREQUEST-STATUS:2.4;Success\r\nEND:VTODO", 1)
	deliverCalendarMail("attendee@localhost", "organizer@localhost", "REPLY", replyTask)
	organizerTask := davE2EExpect(t, davE2ERequest(client, http.MethodGet, taskURL, token, "", "", nil), http.StatusOK)
	decodedTask, err := ical.NewDecoder(strings.NewReader(organizerTask.body)).Decode()
	if err != nil {
		t.Fatal(err)
	}
	taskStatus := decodedTask.Children[0].Props.Values("ATTENDEE")[0].Params.Get("SCHEDULE-STATUS")
	if !strings.Contains(organizerTask.body, "PARTSTAT=ACCEPTED") || taskStatus != "2.0,2.4" {
		t.Fatalf("VTODO reply was not applied: %s", organizerTask.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, taskURL, token, "", "", nil), http.StatusNoContent)
	cancelTask := strings.Replace(requestTask, "METHOD:REQUEST", "METHOD:CANCEL", 1)
	cancelTask = strings.Replace(cancelTask, "SEQUENCE:0", "SEQUENCE:1", 1)
	deliverCalendarMail("organizer@localhost", "attendee@localhost", "CANCEL", cancelTask)
	for attempt := 0; attempt < 20; attempt++ {
		result := davE2ERequest(client, http.MethodGet, attendeeTaskURL, attendeeToken, "", "", nil)
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.status == http.StatusNotFound {
			break
		}
		time.Sleep(100 * time.Millisecond)
		davE2EExpect(t, davE2ERequest(client, http.MethodPost, processExchangeURL, adminToken,
			"application/json", `{"attributes":{}}`, nil), http.StatusOK)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, attendeeTaskURL, attendeeToken, "", "", nil), http.StatusNotFound)
	forceBase := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerSeriesURL, token, "", "", nil), http.StatusOK)
	forced := strings.Replace(forceBase.body, "ATTENDEE;", "ATTENDEE;SCHEDULE-FORCE-SEND=REQUEST;", 1)
	messageCount := len(accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK)))
	for attempt := 1; attempt <= 2; attempt++ {
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, organizerSeriesURL, token, "text/calendar", forced, nil), http.StatusCreated)
		stored := davE2EExpect(t, davE2ERequest(client, http.MethodGet, organizerSeriesURL, token, "", "", nil), http.StatusOK)
		if strings.Contains(stored.body, "SCHEDULE-FORCE-SEND") {
			t.Fatalf("force-send leaked into stored event: %s", stored.body)
		}
		messages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
			activeBase+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
		if len(messages) != messageCount+attempt {
			t.Fatalf("force-send attempt %d queued %d messages, want %d", attempt, len(messages), messageCount+attempt)
		}
	}
	agentEvent := strings.Replace(create, "UID:itip-outbound-e2e", "UID:itip-agent-transition", 1)
	agentURL := activeBase + calendarPath + "agent-transition.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, agentURL, token, "text/calendar", agentEvent, nil), http.StatusCreated)
	clientAgent := strings.Replace(agentEvent, "ATTENDEE;PARTSTAT=NEEDS-ACTION:", "ATTENDEE;PARTSTAT=NEEDS-ACTION;SCHEDULE-AGENT=CLIENT:", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, agentURL, token, "text/calendar", clientAgent, nil), http.StatusCreated)
	agentMessages := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, client, http.MethodGet,
		activeBase+"/api/cal_mail?page%5Bsize%5D=100", adminToken, nil, http.StatusOK))
	methods := make(map[string]int)
	for _, item := range agentMessages {
		attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attrs["uid"] == "itip-agent-transition" {
			methods[fmt.Sprint(attrs["method"])]++
		}
	}
	if methods["REQUEST"] != 1 || methods["CANCEL"] != 1 {
		t.Fatalf("SERVER to CLIENT transition did not cancel the prior invitation: %v", methods)
	}
}
