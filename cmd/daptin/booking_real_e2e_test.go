package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/google/uuid"
)

func TestPublicBookingRealE2E(t *testing.T) {
	requireRealE2E(t)
	runPublicBookingRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "booking.db"))
}

func TestPublicBookingPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runPublicBookingRealE2E(t, "postgres", dsn)
}

func runPublicBookingRealE2E(t *testing.T, databaseType, connectionString string) {
	schema := fmt.Sprintf(`Tables:
  - TableName: collection
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: calendar
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    Metering:
      Enabled: true
      MeterType: requests
      CostExpr: "1"
  - TableName: bookable
    Permission: %d
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: booking
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: user_account
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
  - TableName: usergroup
    Permission: %d
    DefaultPermission: %d
`, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD, auth.UserCRUD, auth.GroupCRUD,
		auth.GuestExecute|auth.UserCRUD, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD,
		auth.UserCRUD, auth.GroupCRUD, auth.GroupRefer, auth.GroupRefer, auth.GuestRefer, auth.GuestRefer)
	usedPorts := map[int]bool{}
	startServer := func() *daptinE2EFixture {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: schema,
		})
		return &daptinE2EFixture{URL: base, Client: &http.Client{Timeout: 20 * time.Second}, Process: process}
	}
	firstServer := startServer()
	admin := firstServer.SignupAdmin(t)
	ftpE2ESetConfig(t, firstServer.Client, firstServer.URL, admin, "caldav.enable", "true")
	firstServer.Process.stopProcess()
	fixture := startServer()
	ownerToken := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "booking-owner")
	principal := davE2EExpect(t, davE2ERequest(fixture.Client, "PROPFIND", fixture.URL+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("owner principal missing: %s", principal.body)
	}
	collectionURL := fixture.URL + "/caldav/" + match[1] + "/calendars/appointments/"
	davE2EExpect(t, davE2ERequest(fixture.Client, "MKCOL", collectionURL, ownerToken, "", "", nil), http.StatusCreated)
	collectionID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "collection", "name", "appointments")
	day := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	hostToken := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "booking-host")
	hostPrincipal := davE2EExpect(t, davE2ERequest(fixture.Client, "PROPFIND", fixture.URL+"/caldav/", hostToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	hostMatch := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(hostPrincipal.body)
	if len(hostMatch) != 2 {
		t.Fatalf("host principal missing: %s", hostPrincipal.body)
	}
	hostURL := fixture.URL + "/caldav/" + hostMatch[1] + "/calendars/host/"
	davE2EExpect(t, davE2ERequest(fixture.Client, "MKCOL", hostURL, hostToken, "", "", nil), http.StatusCreated)
	privateEvent := fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//Booking Test//EN\r\nBEGIN:VEVENT\r\nUID:host-private\r\nDTSTAMP:%sT000000Z\r\nDTSTART:%sT120000Z\r\nDTEND:%sT130000Z\r\nSUMMARY:Private host details\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", strings.ReplaceAll(day, "-", ""), strings.ReplaceAll(day, "-", ""), strings.ReplaceAll(day, "-", ""))
	davE2EExpect(t, davE2ERequest(fixture.Client, http.MethodPut, hostURL+"busy.ics", hostToken, "text/calendar", privateEvent, nil), http.StatusCreated)
	hostCollectionID := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "collection", "name", "host")
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/collection/share_user", hostToken,
		map[string]interface{}{"calendar_reference_id": hostCollectionID, "user_account_id": match[1], "permission": int64(auth.GroupPeek | auth.GroupRefer)}, http.StatusOK)
	hours := `{"sun":[{"Start":"09:00","End":"17:00"}],"mon":[{"Start":"09:00","End":"17:00"}],"tue":[{"Start":"09:00","End":"17:00"}],"wed":[{"Start":"09:00","End":"17:00"}],"thu":[{"Start":"09:00","End":"17:00"}],"fri":[{"Start":"09:00","End":"17:00"}],"sat":[{"Start":"09:00","End":"17:00"}]}`
	bookableID := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, ownerToken, "bookable", map[string]interface{}{
		"title": "Consultation", "published": true,
		"destination_collection_id": collectionID, "time_zone": "UTC", "weekly_hours": hours,
		"duration_minutes": 30, "increment_minutes": 30, "buffer_before_minutes": 0,
		"buffer_after_minutes": 0, "minimum_notice_minutes": 0, "horizon_days": 30,
		"capacity": 1, "daily_limit": 0, "weekly_limit": 0,
	})
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", hostToken,
		map[string]interface{}{"bookable_ref": bookableID, "calendar_ref": hostCollectionID, "enabled": true}, http.StatusForbidden)
	for _, calendarID := range []string{collectionID, hostCollectionID} {
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", ownerToken,
			map[string]interface{}{"bookable_ref": bookableID, "calendar_ref": calendarID, "enabled": true}, http.StatusOK)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", ownerToken,
		map[string]interface{}{"bookable_ref": bookableID, "calendar_ref": hostCollectionID}, http.StatusBadRequest)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/collection/share_user", hostToken,
		map[string]interface{}{"calendar_reference_id": hostCollectionID, "user_account_id": match[1], "permission": int64(auth.GroupPeek)}, http.StatusOK)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/slots", "",
		map[string]interface{}{"bookable_ref": bookableID, "date": day}, http.StatusConflict)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/collection/share_user", hostToken,
		map[string]interface{}{"calendar_reference_id": hostCollectionID, "user_account_id": match[1], "permission": int64(auth.GroupPeek | auth.GroupRefer)}, http.StatusOK)
	slots := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/slots", "",
		map[string]interface{}{"bookable_ref": bookableID, "date": day}, http.StatusOK)
	if strings.Contains(fmt.Sprint(slots), "VEVENT") {
		t.Fatalf("public slots leaked event data: %#v", slots)
	}
	if strings.Contains(fmt.Sprint(slots), "Private host details") || strings.Contains(fmt.Sprint(slots), day+"T12:00") {
		t.Fatalf("host calendar details or busy time leaked: %#v", slots)
	}
	usageRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		fixture.URL+"/api/api_usage?page%5Bsize%5D=100", admin, nil, http.StatusOK))
	meteredOwnerRead := false
	for _, item := range usageRows {
		attributes := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attributes["entity_type"] == "calendar" && attributes["method"] == http.MethodGet &&
			attributes["endpoint"] == "/api/calendar" && attributes["user_account_id"] == match[1] && attributes["state"] == "completed" {
			meteredOwnerRead = true
		}
	}
	if !meteredOwnerRead {
		t.Fatalf("public availability read was not metered to the bookable owner: %#v", usageRows)
	}
	result := slots.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	available := result["slots"].([]interface{})
	if len(available) == 0 {
		t.Fatalf("no slots returned: %#v", slots)
	}
	start := available[0].(string)
	key := uuid.NewString()
	input := map[string]interface{}{"bookable_ref": bookableID, "start": start, "guest_name": "Guest One", "guest_email": "guest@example.net", "attempt_key": key}
	first := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "", input, http.StatusOK)
	retry := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "", input, http.StatusOK)
	if fmt.Sprint(first) != fmt.Sprint(retry) {
		t.Fatalf("retry changed booking: first=%#v retry=%#v", first, retry)
	}
	changedAttempt := map[string]interface{}{"bookable_ref": bookableID, "start": start, "guest_name": "Changed Guest",
		"guest_email": "guest@example.net", "attempt_key": key}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "", changedAttempt, http.StatusConflict)
	accessGroupsE2EAssertListCount(t, fixture.Client, fixture.URL, admin, "booking", 1)
	confirmed := first.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	bookingID, _ := confirmed["booking_ref"].(string)
	token, _ := confirmed["token"].(string)
	if bookingID == "" || bookingID == "00000000-0000-0000-0000-000000000000" || token == "" {
		t.Fatalf("reservation has no stable booking identity: %#v", first)
	}
	conflict := map[string]interface{}{"bookable_ref": bookableID, "start": start, "guest_name": "Guest Two", "guest_email": "second@example.net", "attempt_key": uuid.NewString()}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "", conflict, http.StatusConflict)
	remaining := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/slots", "",
		map[string]interface{}{"bookable_ref": bookableID, "date": day}, http.StatusOK)
	if strings.Contains(fmt.Sprint(remaining), start) {
		t.Fatalf("reserved slot still public: %#v", remaining)
	}
	status := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/status", "", map[string]interface{}{"booking_ref": bookingID, "token": token}, http.StatusOK)
	if !strings.Contains(fmt.Sprint(status), "confirmed") {
		t.Fatalf("status missing confirmation: %#v", status)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/status", "", map[string]interface{}{"booking_ref": bookingID, "token": "wrong"}, http.StatusForbidden)
	calendarListing := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/calendar?page%5Bsize%5D=100", admin, nil, http.StatusOK)
	eventRows := accessGroupsE2EDataArray(t, calendarListing)
	if len(eventRows) != 2 {
		t.Fatalf("reservation produced %d total calendar events", len(eventRows))
	}
	var event map[string]interface{}
	for _, item := range eventRows {
		candidate := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if strings.Contains(candidate["rpath"].(string), "/appointments/") {
			event = candidate
		}
	}
	if event == nil {
		t.Fatalf("booking event missing: %#v", calendarListing)
	}
	eventPath := event["rpath"].(string)
	if response := davE2ERequest(fixture.Client, http.MethodPut, fixture.URL+eventPath, ownerToken, "text/calendar", davE2ECalendarA, nil); response.err != nil || response.status != http.StatusConflict {
		t.Fatalf("DAV changed booking event: %+v", response)
	}
	if len(available) < 2 {
		t.Fatalf("need a second slot for reschedule: %#v", slots)
	}
	newStart := available[1].(string)
	rescheduled := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reschedule", "",
		map[string]interface{}{"booking_ref": bookingID, "token": token, "start": newStart}, http.StatusOK)
	if !strings.Contains(fmt.Sprint(rescheduled), newStart) {
		t.Fatalf("reschedule did not move event: %#v", rescheduled)
	}
	rescheduledRow := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/booking/"+bookingID, admin, nil, http.StatusOK)
	rescheduledAttributes := rescheduledRow.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	storedStart, parseErr := time.Parse(time.RFC3339, rescheduledAttributes["starts_at"].(string))
	wantedStart, _ := time.Parse(time.RFC3339, newStart)
	if parseErr != nil || !storedStart.Equal(wantedStart) {
		t.Fatalf("reschedule did not persist booking time: %#v", rescheduledRow)
	}
	updatedEvent := davE2EExpect(t, davE2ERequest(fixture.Client, http.MethodGet, fixture.URL+eventPath, ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(updatedEvent.body, "DTSTART:"+strings.ReplaceAll(strings.ReplaceAll(newStart, "-", ""), ":", "")[:15]) {
		t.Fatalf("CalDAV event did not move: %s", updatedEvent.body)
	}
	cancelled := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/cancel", "",
		map[string]interface{}{"booking_ref": bookingID, "token": token}, http.StatusOK)
	if !strings.Contains(fmt.Sprint(cancelled), "cancelled") {
		t.Fatalf("cancel failed: %#v", cancelled)
	}
	cancelledEvent := davE2EExpect(t, davE2ERequest(fixture.Client, http.MethodGet, fixture.URL+eventPath, ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(cancelledEvent.body, "STATUS:CANCELLED") {
		t.Fatalf("event was not cancelled: %s", cancelledEvent.body)
	}
	storedBooking := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/booking/"+bookingID, admin, nil, http.StatusOK)
	storedAttributes := storedBooking.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if storedAttributes["state"] != "cancelled" {
		t.Fatalf("cancelled booking did not persist: %#v", storedBooking)
	}
	requests := []string{
		fmt.Sprintf(`{"bookable_ref":%q,"start":%q,"guest_name":"Concurrent One","guest_email":"one@example.net","attempt_key":%q}`, bookableID, start, uuid.NewString()),
		fmt.Sprintf(`{"bookable_ref":%q,"start":%q,"guest_name":"Concurrent Two","guest_email":"two@example.net","attempt_key":%q}`, bookableID, start, uuid.NewString()),
	}
	results := make([]davE2EResponse, 2)
	var wait sync.WaitGroup
	for i := range requests {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index] = davE2ERequest(fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "", "application/json", requests[index], nil)
		}(i)
	}
	wait.Wait()
	successes, conflicts := 0, 0
	for _, result := range results {
		if result.err != nil {
			t.Fatal(result.err)
		}
		switch result.status {
		case http.StatusOK:
			successes++
		case http.StatusConflict:
			conflicts++
		default:
			t.Fatalf("concurrent booking returned %d: %s", result.status, result.body)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent last slot: %+v", results)
	}
	groupID := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, ownerToken, "bookable", map[string]interface{}{
		"title": "Group session", "published": true,
		"destination_collection_id": collectionID, "time_zone": "UTC", "weekly_hours": hours,
		"questions":        `[{"id":"topic","label":"Topic","required":true}]`,
		"duration_minutes": 30, "increment_minutes": 30, "buffer_before_minutes": 15,
		"buffer_after_minutes": 15, "minimum_notice_minutes": 0, "horizon_days": 30,
		"capacity": 2, "daily_limit": 2, "weekly_limit": 2,
	})
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", ownerToken,
		map[string]interface{}{"bookable_ref": groupID, "calendar_ref": collectionID, "enabled": true}, http.StatusOK)
	groupStart := day + "T10:00:00Z"
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "",
		map[string]interface{}{"bookable_ref": groupID, "start": groupStart, "guest_name": "Missing Answer",
			"guest_email": "missing@example.net", "attempt_key": uuid.NewString()}, http.StatusBadRequest)
	groupBookings := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		groupBooking := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "",
			map[string]interface{}{"bookable_ref": groupID, "start": groupStart, "guest_name": fmt.Sprintf("Group %d", i),
				"guest_email": fmt.Sprintf("group%d@example.net", i), "attempt_key": uuid.NewString(), "answers": map[string]interface{}{"topic": "planning"}}, http.StatusOK)
		groupBookings = append(groupBookings, groupBooking.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})["booking_ref"].(string))
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "",
		map[string]interface{}{"bookable_ref": groupID, "start": groupStart, "guest_name": "Group Three",
			"guest_email": "group3@example.net", "attempt_key": uuid.NewString(), "answers": map[string]interface{}{"topic": "planning"}}, http.StatusConflict)
	groupSlots := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/slots", "",
		map[string]interface{}{"bookable_ref": groupID, "date": day}, http.StatusOK)
	if strings.Contains(fmt.Sprint(groupSlots), groupStart) || strings.Contains(fmt.Sprint(groupSlots), day+"T10:30:00Z") {
		t.Fatalf("capacity or buffer did not close group slots: %#v", groupSlots)
	}
	groupAttrs := groupSlots.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	if len(groupAttrs["slots"].([]interface{})) != 0 {
		t.Fatalf("daily limit left slots available: %#v", groupSlots)
	}
	ownerCancelled := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/cancel", ownerToken,
		map[string]interface{}{"booking_ref": groupBookings[0]}, http.StatusOK)
	if !strings.Contains(fmt.Sprint(ownerCancelled), "cancelled") {
		t.Fatalf("owner could not cancel booking: %#v", ownerCancelled)
	}
	mailServer := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_server", map[string]interface{}{
		"hostname": "localhost", "is_enabled": false, "listen_interface": "127.0.0.1:0",
		"max_size": 10000, "max_clients": 1, "xclient_on": false,
		"always_on_tls": false, "authentication_required": false,
	})
	mailAccount := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "mail_account", map[string]interface{}{
		"username": "bookings@localhost", "password": "testpass123", "password_md5": "testpass123", "mail_server_id": mailServer,
	})
	certificate := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, admin, "certificate", map[string]interface{}{
		"hostname": "localhost", "issuer": "self",
	})
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/certificate/generate_self_certificate", admin,
		map[string]interface{}{"attributes": map[string]interface{}{"certificate_id": certificate}}, http.StatusOK)
	davE2EExpect(t, davE2ERequest(fixture.Client, http.MethodPatch,
		fixture.URL+"/api/collection/"+collectionID+"/relationships/scheduling_mail_account_id", admin,
		"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":%q}}`, mailAccount), nil), http.StatusNoContent)
	connected := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reserve", "",
		map[string]interface{}{"bookable_ref": bookableID, "start": day + "T11:00:00Z", "guest_name": "Invited Guest",
			"guest_email": "invited@example.net", "attempt_key": uuid.NewString()}, http.StatusOK)
	connectedAttrs := connected.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	connectedID := connectedAttrs["booking_ref"].(string)
	connectedToken := connectedAttrs["token"].(string)
	connectedStatus := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/status", "",
		map[string]interface{}{"booking_ref": connectedID, "token": connectedToken}, http.StatusOK)
	if !strings.Contains(fmt.Sprint(connectedStatus), "submitted") || !strings.Contains(fmt.Sprint(connectedStatus), "mail_configured:true") {
		t.Fatalf("connected booking did not expose queued invitation: %#v", connectedStatus)
	}
	connectedRow := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/booking/"+connectedID, admin, nil, http.StatusOK)
	connectedEventID := connectedRow.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})["calendar_id"].(string)
	connectedEventRow := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, fixture.URL+"/api/calendar/"+connectedEventID, admin, nil, http.StatusOK)
	connectedPath := connectedEventRow.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})["rpath"].(string)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/reschedule", "",
		map[string]interface{}{"booking_ref": connectedID, "token": connectedToken, "start": day + "T11:30:00Z"}, http.StatusOK)
	connectedEvent := davE2EExpect(t, davE2ERequest(fixture.Client, http.MethodGet, fixture.URL+connectedPath, ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(connectedEvent.body, "SEQUENCE:1") || !strings.Contains(connectedEvent.body, "SCHEDULE-STATUS=1.0") || connectedEvent.header.Get("Schedule-Tag") == "" {
		t.Fatalf("rescheduled invitation metadata missing: %+v", connectedEvent)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/cancel", "",
		map[string]interface{}{"booking_ref": connectedID, "token": connectedToken}, http.StatusOK)
	mailRows := accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		fixture.URL+"/api/cal_mail?page%5Bsize%5D=100", admin, nil, http.StatusOK))
	if len(mailRows) < 3 {
		t.Fatalf("booking invitation, reschedule, and cancellation were not queued: %#v", mailRows)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", ownerToken,
		map[string]interface{}{"bookable_ref": bookableID, "calendar_ref": hostCollectionID, "enabled": false}, http.StatusOK)
	withoutHost := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/slots", "",
		map[string]interface{}{"bookable_ref": bookableID, "date": day}, http.StatusOK)
	if !strings.Contains(fmt.Sprint(withoutHost), day+"T12:00:00Z") {
		t.Fatalf("removing a required host did not update availability: %#v", withoutHost)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, fixture.URL+"/api/bookable/"+bookableID, ownerToken,
		accessGroupsE2ERecordPayload("bookable", bookableID, map[string]interface{}{"published": false}), http.StatusOK)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/slots", "",
		map[string]interface{}{"bookable_ref": bookableID, "date": day}, http.StatusNotFound)
	fixture.Process.stopProcess()
	restarted := startServer()
	retried := accessGroupsE2ERequestJSON(t, restarted.Client, http.MethodPost, restarted.URL+"/action/bookable/reserve", "", input, http.StatusOK)
	if !strings.Contains(fmt.Sprint(retried), bookingID) || !strings.Contains(fmt.Sprint(retried), "cancelled") {
		t.Fatalf("retry after restart did not return original booking: %#v", retried)
	}
	accessGroupsE2ERequestJSON(t, restarted.Client, http.MethodPost, restarted.URL+"/action/bookable/reserve", "",
		map[string]interface{}{"bookable_ref": bookableID, "start": start, "guest_name": "Late Guest",
			"guest_email": "late@example.net", "attempt_key": uuid.NewString()}, http.StatusNotFound)
	accessGroupsE2ERequestJSON(t, restarted.Client, http.MethodPatch, restarted.URL+"/api/bookable/"+bookableID, admin,
		accessGroupsE2ERecordPayload("bookable", bookableID, map[string]interface{}{"permission": 0}), http.StatusOK)
	accessGroupsE2ERequestJSON(t, restarted.Client, http.MethodPost, restarted.URL+"/action/bookable/set_calendar", ownerToken,
		map[string]interface{}{"bookable_ref": bookableID, "calendar_ref": collectionID, "enabled": false}, http.StatusForbidden)
}
