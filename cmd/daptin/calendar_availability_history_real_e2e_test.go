package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
	"github.com/google/uuid"
)

func TestCalendarAvailabilityHistoryRealE2E(t *testing.T) {
	requireRealE2E(t)
	databaseType := "sqlite3"
	connectionString := filepath.Join(t.TempDir(), "calendar-history.db")
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

	hours := `{"sun":[{"Start":"09:00","End":"17:00"}],"mon":[{"Start":"09:00","End":"17:00"}],"tue":[{"Start":"09:00","End":"17:00"}],"wed":[{"Start":"09:00","End":"17:00"}],"thu":[{"Start":"09:00","End":"17:00"}],"fri":[{"Start":"09:00","End":"17:00"}],"sat":[{"Start":"09:00","End":"17:00"}]}`
	offer := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, ownerToken, "bookable", map[string]interface{}{
		"title": "Calendar history", "published": true, "destination_collection_id": collectionID, "time_zone": "UTC", "weekly_hours": hours,
		"duration_minutes": 60, "increment_minutes": 60, "buffer_before_minutes": 0, "buffer_after_minutes": 0, "minimum_notice_minutes": 0, "horizon_days": 30, "capacity": 1, "daily_limit": 0, "weekly_limit": 0,
	})
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", ownerToken, map[string]interface{}{"bookable_ref": offer, "calendar_ref": collectionID, "enabled": true}, http.StatusOK)
	event := func(uid string, when time.Time) string {
		return fmt.Sprintf("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//History Test//EN\r\nBEGIN:VEVENT\r\nUID:%s\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:%s\r\nDTEND:%s\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n", uid, when.Format("20060102T150405Z"), when.Add(time.Hour).Format("20060102T150405Z"))
	}
	target, _ := time.Parse("2006-01-02", day)
	davE2EExpect(t, davE2ERequest(fixture.Client, "PUT", collectionURL+"target.ics", ownerToken, "text/calendar", event("target", target.Add(15*time.Hour)), nil), 201)
	slots := func(want int) []interface{} {
		response := accessGroupsE2ERequestJSON(t, fixture.Client, "POST", fixture.URL+"/action/bookable/slots", "", map[string]interface{}{"bookable_ref": offer, "date": day}, want)
		if want != http.StatusOK {
			return nil
		}
		return response.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})["slots"].([]interface{})
	}
	assertSlots := func(want int) {
		t.Helper()
		got := slots(http.StatusOK)
		if len(got) != want || strings.Contains(fmt.Sprint(got), day+"T15:00:00Z") {
			t.Fatalf("unexpected available slots: %v", got)
		}
	}
	report := fmt.Sprintf(`<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="%s" end="%s"/></C:free-busy-query>`, target.Format("20060102T150405Z"), target.Add(24*time.Hour).Format("20060102T150405Z"))
	freebusy := func(status int) {
		res := davE2EExpect(t, davE2ERequest(fixture.Client, "REPORT", collectionURL, ownerToken, "application/xml", report, http.Header{"Depth": {"1"}}), status)
		if status == http.StatusOK && !strings.Contains(res.body, "FREEBUSY:"+target.Add(15*time.Hour).Format("20060102T150405Z")+"/"+target.Add(16*time.Hour).Format("20060102T150405Z")) {
			t.Fatalf("missing target busy interval: %s", res.body)
		}
	}
	assertSlots(7)
	freebusy(200)
	for i := 0; i < 1000; i++ {
		uid := fmt.Sprintf("past-%d", i)
		davE2EExpect(t, davE2ERequest(fixture.Client, "PUT", collectionURL+uid+".ics", ownerToken, "text/calendar", event(uid, time.Date(2020, 1, 1, 12, 0, 0, 0, time.UTC).AddDate(0, 0, i)), nil), 201)
		if (i+1)%250 == 0 {
			t.Logf("created %d historical events", i+1)
		}
	}
	assertSlots(7)
	freebusy(200)
	query := fmt.Sprintf(`<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"><C:time-range start="%s" end="%s"/></C:comp-filter></C:comp-filter></C:filter></C:calendar-query>`, target.Format("20060102T150405Z"), target.Add(24*time.Hour).Format("20060102T150405Z"))
	res := davE2EExpect(t, davE2ERequest(fixture.Client, "REPORT", collectionURL, ownerToken, "application/xml", query, nil), 207)
	if !strings.Contains(res.body, "target.ics") || strings.Contains(res.body, "past-") {
		t.Fatalf("range query wrong: %s", res.body)
	}

	// An old DTSTART still contributes if its recurring instances overlap the window.
	recurring := strings.Replace(event("recurring", time.Date(2020, 1, 1, 14, 0, 0, 0, time.UTC)), "END:VEVENT", "RRULE:FREQ=DAILY\r\nEND:VEVENT", 1)
	davE2EExpect(t, davE2ERequest(fixture.Client, "PUT", collectionURL+"recurring.ics", ownerToken, "text/calendar", recurring, nil), 201)
	assertSlots(6)
	// Reservation and rescheduling must reuse the same history-safe availability path.
	available := slots(200)
	reservation := accessGroupsE2ERequestJSON(t, fixture.Client, "POST", fixture.URL+"/action/bookable/reserve", "", map[string]interface{}{"bookable_ref": offer, "start": available[0], "guest_name": "History Guest", "guest_email": "history@example.net", "attempt_key": uuid.NewString()}, 200)
	attrs := reservation.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	assertSlots(5)
	accessGroupsE2ERequestJSON(t, fixture.Client, "POST", fixture.URL+"/action/bookable/reschedule", "", map[string]interface{}{"booking_ref": attrs["booking_ref"], "token": attrs["token"], "start": available[1]}, 200)
	assertSlots(5)
	freebusy(200)
	accessGroupsE2ERequestJSON(t, fixture.Client, "POST", fixture.URL+"/action/bookable/cancel", "", map[string]interface{}{"booking_ref": attrs["booking_ref"], "token": attrs["token"]}, 200)
	assertSlots(6)
	// Exactly 1000 contributing objects are allowed; one more must fail closed.
	davE2EExpect(t, davE2ERequest(fixture.Client, "DELETE", collectionURL+"recurring.ics", ownerToken, "", "", nil), 204)
	for i := 0; i < 999; i++ {
		uid := fmt.Sprintf("past-%d", i)
		davE2EExpect(t, davE2ERequest(fixture.Client, "PUT", collectionURL+uid+".ics", ownerToken, "text/calendar", event(uid, target.Add(15*time.Hour)), nil), 201)
	}
	assertSlots(7)
	freebusy(200)
	// A confirmed booking at the object limit is accounted for by booking capacity,
	// not charged again as a calendar candidate.
	reservation = accessGroupsE2ERequestJSON(t, fixture.Client, "POST", fixture.URL+"/action/bookable/reserve", "", map[string]interface{}{"bookable_ref": offer, "start": slots(200)[0], "guest_name": "Limit Guest", "guest_email": "limit@example.net", "attempt_key": uuid.NewString()}, 200)
	attrs = reservation.([]interface{})[0].(map[string]interface{})["Attributes"].(map[string]interface{})
	assertSlots(6)
	freebusy(507)
	accessGroupsE2ERequestJSON(t, fixture.Client, "POST", fixture.URL+"/action/bookable/cancel", "", map[string]interface{}{"booking_ref": attrs["booking_ref"], "token": attrs["token"]}, 200)
	assertSlots(7)
	freebusy(200)
	davE2EExpect(t, davE2ERequest(fixture.Client, "PUT", collectionURL+"past-999.ics", ownerToken, "text/calendar", event("past-999", target.Add(15*time.Hour)), nil), 201)
	slots(409)
	freebusy(507)
}
