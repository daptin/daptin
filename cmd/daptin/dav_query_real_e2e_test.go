package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDAVCalendarQuerySelectionRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav-query.db")
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
	admin := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, admin, "caldav.enable", "true")
	first.stopProcess()
	base, process := start()
	defer process.stopProcess()
	owner := accessGroupsE2ESignupSigninUser(t, client, base, admin, "dav-query-owner")
	outsider := accessGroupsE2ESignupSigninUser(t, client, base, admin, "dav-query-outsider")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", owner,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("missing owner principal: %s", principal.body)
	}
	collection := base + "/caldav/" + match[1] + "/calendars/query/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", collection, owner, "", "", nil), http.StatusCreated)
	task := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VTODO\r\nUID:query-task\r\nDTSTAMP:20261009T000000Z\r\nDUE:20261010T120000Z\r\nSUMMARY:Team Meeting\r\nLOCATION:Private room\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, collection+"task.ics", owner, "text/calendar", task, nil), http.StatusCreated)
	properties := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collection, owner, "application/xml",
		`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:supported-collation-set/></D:prop></D:propfind>`, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	if !strings.Contains(properties.body, "i;ascii-casemap") || !strings.Contains(properties.body, "i;octet") {
		t.Fatalf("missing supported collations: %s", properties.body)
	}
	query := `<C:calendar-query xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><D:getetag/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VTODO"><C:time-range start="20261010T000000Z" end="20261011T000000Z"/><C:prop-filter name="SUMMARY"><C:text-match>meeting</C:text-match></C:prop-filter></C:comp-filter></C:comp-filter></C:filter></C:calendar-query>`
	result := davE2EExpect(t, davE2ERequest(client, "REPORT", collection, owner, "application/xml", query, nil), http.StatusMultiStatus)
	if !strings.Contains(result.body, "task.ics") {
		t.Fatalf("task time range and default collation did not match: %s", result.body)
	}
	octetQuery := strings.Replace(query, `<C:text-match>meeting</C:text-match>`, `<C:text-match collation="i;octet">meeting</C:text-match>`, 1)
	result = davE2EExpect(t, davE2ERequest(client, "REPORT", collection, owner, "application/xml", octetQuery, nil), http.StatusMultiStatus)
	if strings.Contains(result.body, "task.ics") {
		t.Fatalf("octet collation matched different case: %s", result.body)
	}
	multiget := `<C:calendar-multiget xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:prop><C:calendar-data><C:comp name="VCALENDAR"><C:prop name="VERSION"/><C:comp name="VTODO"><C:prop name="SUMMARY"/></C:comp></C:comp></C:calendar-data></D:prop><D:href>/caldav/` + match[1] + `/calendars/query/task.ics</D:href></C:calendar-multiget>`
	selected := davE2EExpect(t, davE2ERequest(client, "REPORT", collection, owner, "application/xml", multiget, nil), http.StatusMultiStatus)
	if !strings.Contains(selected.body, "Team Meeting") || strings.Contains(selected.body, "Private room") || strings.Contains(selected.body, "UID:query-task") {
		t.Fatalf("partial calendar-data response is wrong: %s", selected.body)
	}
	noValue := strings.Replace(multiget, `<C:prop name="SUMMARY"/>`, `<C:prop name="SUMMARY" novalue="yes"/>`, 1)
	selected = davE2EExpect(t, davE2ERequest(client, "REPORT", collection, owner, "application/xml", noValue, nil), http.StatusMultiStatus)
	if !strings.Contains(selected.body, "SUMMARY:") || strings.Contains(selected.body, "Team Meeting") {
		t.Fatalf("novalue calendar-data response is wrong: %s", selected.body)
	}
	propfindData := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collection+"task.ics", owner, "application/xml",
		`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(propfindData.body, "404 Not Found") || strings.Contains(propfindData.body, "Team Meeting") {
		t.Fatalf("calendar-data was returned as a PROPFIND property: %s", propfindData.body)
	}
	expand := strings.Replace(multiget, `</C:calendar-data>`, `<C:expand start="20261010T000000Z" end="20261011T000000Z"/></C:calendar-data>`, 1)
	unsupported := davE2ERequest(client, "REPORT", collection, owner, "application/xml", expand, nil)
	if unsupported.err != nil || unsupported.status != http.StatusMultiStatus || !strings.Contains(unsupported.body, "501 Not Implemented") {
		t.Fatalf("unsupported recurrence expansion returned %+v", unsupported)
	}
	recurrence := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VTODO\r\nUID:query-recurring-task\r\nDTSTAMP:20261009T000000Z\r\nDTSTART:20261008T120000Z\r\nDUE:20261008T130000Z\r\nRRULE:FREQ=DAILY;COUNT=4\r\nSUMMARY:Recurring task\r\nEND:VTODO\r\nEND:VCALENDAR\r\n"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, collection+"recurring.ics", owner, "text/calendar", recurrence, nil), http.StatusCreated)
	recurringQuery := strings.Replace(query, `<C:prop-filter name="SUMMARY"><C:text-match>meeting</C:text-match></C:prop-filter>`, "", 1)
	result = davE2EExpect(t, davE2ERequest(client, "REPORT", collection, owner, "application/xml", recurringQuery, nil), http.StatusMultiStatus)
	if !strings.Contains(result.body, "recurring.ics") {
		t.Fatalf("recurring task did not match time range: %s", result.body)
	}
	if denied := davE2ERequest(client, "REPORT", collection, outsider, "application/xml", query, nil); denied.err != nil || denied.status == http.StatusMultiStatus && strings.Contains(denied.body, "Team Meeting") {
		t.Fatalf("outsider read query data: %+v", denied)
	}
}
