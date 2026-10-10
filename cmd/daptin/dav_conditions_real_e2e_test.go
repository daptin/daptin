package main

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

var davE2EAccessSchema = fmt.Sprintf(`Tables:
  - TableName: calendar
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    Metering:
      Enabled: true
      MeterType: requests
      CostExpr: "1"
  - TableName: collection
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: address_book
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: contact
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: usergroup
    Permission: %d
    DefaultPermission: %d
`, auth.UserCRUD, auth.GroupCRUD, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD|auth.GroupExecute,
	auth.UserCRUD|auth.UserExecute, auth.GroupCRUD|auth.GroupExecute, auth.UserCRUD, auth.GroupCRUD,
	auth.GuestRefer, auth.GuestRefer,
)

const davE2ECalendarA = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:dav-condition-test\r\nDTSTAMP:20261005T120000Z\r\nDTSTART:20261006T120000Z\r\nDTEND:20261006T130000Z\r\nSUMMARY:First\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
const davE2ECalendarB = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:dav-condition-test\r\nDTSTAMP:20261005T120000Z\r\nDTSTART:20261006T120000Z\r\nDTEND:20261006T130000Z\r\nSUMMARY:Second\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
const davE2ERecurringCalendar = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:long-recurring\r\nDTSTAMP:20261005T000000Z\r\nDTSTART:20261001T000000Z\r\nDTEND:20261004T000000Z\r\nRRULE:FREQ=WEEKLY;COUNT=4\r\nSUMMARY:Long event\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
const davE2ECardA = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:dav-condition-test\r\nFN:First\r\nEND:VCARD\r\n"
const davE2ECardB = "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:dav-condition-test\r\nFN:Second\r\nEND:VCARD\r\n"

func TestDAVRequiresConfiguredAccessRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav-unconfigured.db")
	client := &http.Client{Timeout: 20 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: "sqlite3", connectionString: databasePath, olricPort: olricPort,
		})
		return base, process
	}
	firstURL, first := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
	first.stopProcess()
	base, second := start()
	defer second.stopProcess()
	token := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-unconfigured")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("principal reference ID missing: %s", principal.body)
	}
	davE2EExpect(t, davE2ERequest(client, "MKCOL", base+"/caldav/"+match[1]+"/calendars/personal/", token,
		"", "", nil), http.StatusForbidden)
}

type davE2EResponse struct {
	status int
	header http.Header
	body   string
	err    error
}

func davE2ERequest(client *http.Client, method, url, token, contentType, body string, headers http.Header) davE2EResponse {
	request, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		return davE2EResponse{err: err}
	}
	request.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	for name, values := range headers {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return davE2EResponse{err: err}
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return davE2EResponse{err: err}
	}
	return davE2EResponse{status: response.StatusCode, header: response.Header, body: string(data)}
}

func davE2EExpect(t *testing.T, response davE2EResponse, status int) davE2EResponse {
	t.Helper()
	if response.err != nil {
		t.Fatal(response.err)
	}
	if response.status != status {
		t.Fatalf("DAV status = %d, want %d; body: %s", response.status, status, response.body)
	}
	return response
}

func davE2ECollectionReferenceID(t *testing.T, response interface{}, name string) string {
	t.Helper()
	root, ok := response.(map[string]interface{})
	if !ok {
		t.Fatalf("collection list is not an object: %#v", response)
	}
	data, ok := root["data"].([]interface{})
	if !ok {
		t.Fatalf("collection list has no data: %#v", response)
	}
	for _, item := range data {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		attributes, _ := row["attributes"].(map[string]interface{})
		if attributes["name"] == name {
			if referenceID, ok := row["id"].(string); ok && referenceID != "" {
				return referenceID
			}
		}
	}
	t.Fatalf("collection %q not found: %#v", name, response)
	return ""
}

func TestDAVConditionalWritesRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav.db")
	client := &http.Client{Timeout: 20 * time.Second}
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
	base, second := start()
	defer second.stopProcess()
	token := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-other")

	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("principal reference ID missing: %s", principal.body)
	}
	userID := match[1]
	basicRequest, err := http.NewRequest("PROPFIND", base+"/caldav/", strings.NewReader(
		`<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`))
	if err != nil {
		t.Fatal(err)
	}
	basicRequest.SetBasicAuth("dav-owner@test.local", "testpass123")
	basicRequest.Header.Set("Content-Type", "application/xml")
	basicResponse, err := client.Do(basicRequest)
	if err != nil {
		t.Fatal(err)
	}
	basicBody, err := io.ReadAll(basicResponse.Body)
	basicResponse.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if basicResponse.StatusCode != http.StatusMultiStatus || !strings.Contains(string(basicBody), "/caldav/"+userID+"/") {
		t.Fatalf("Basic authentication discovered a different principal: %d %s", basicResponse.StatusCode, basicBody)
	}
	otherPrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", otherToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if strings.Contains(otherPrincipal.body, "/caldav/"+userID+"/") {
		t.Fatalf("different users discovered the same DAV principal: %s", otherPrincipal.body)
	}

	tests := []struct {
		name, collectionURL, collectionName, collectionTable, descriptionNamespace, descriptionProperty string
		objectName, contentType, first, second                                                          string
	}{
		{"calendar", base + "/caldav/" + userID + "/calendars/personal/", "personal", "collection",
			"urn:ietf:params:xml:ns:caldav", "calendar-description", "event.ics", "text/calendar", davE2ECalendarA, davE2ECalendarB},
		{"address book", base + "/carddav/" + userID + "/addressbooks/contacts/", "contacts", "address_book",
			"urn:ietf:params:xml:ns:carddav", "addressbook-description", "person.vcf", "text/vcard", davE2ECardA, davE2ECardB},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			davE2EExpect(t, davE2ERequest(client, "MKCOL", test.collectionURL, token, "", "", nil), http.StatusCreated)
			referenceID := davE2ECollectionReferenceID(t,
				transportE2EGetJSON(t, client, base+"/api/"+test.collectionTable, adminToken), test.collectionName)
			patchURL := base + "/api/" + test.collectionTable + "/" + referenceID
			payload := fmt.Sprintf(`{"data":{"type":"%s","id":"%s","attributes":{"description":"Work collection"}}}`,
				test.collectionTable, referenceID)
			davE2EExpect(t, davE2ERequest(client, http.MethodPatch, patchURL, adminToken,
				"application/vnd.api+json", payload, nil), http.StatusOK)
			ownerPayload := fmt.Sprintf(`{"data":{"type":"%s","id":"%s","attributes":{"description":"Owner change"}}}`,
				test.collectionTable, referenceID)
			davE2EExpect(t, davE2ERequest(client, http.MethodPatch, patchURL, token,
				"application/vnd.api+json", ownerPayload, nil), http.StatusOK)
			expectedDescription := "Owner change"
			propertyRequest := fmt.Sprintf(`<D:propfind xmlns:D="DAV:" xmlns:C="%s"><D:prop><C:%s/></D:prop></D:propfind>`,
				test.descriptionNamespace, test.descriptionProperty)
			properties := davE2EExpect(t, davE2ERequest(client, "PROPFIND", test.collectionURL, token,
				"application/xml", propertyRequest, nil), http.StatusMultiStatus)
			if !strings.Contains(properties.body, expectedDescription) {
				t.Fatalf("DAV did not read the resource description: %s", properties.body)
			}
			propertyUpdate := fmt.Sprintf(`<D:propertyupdate xmlns:D="DAV:" xmlns:C="%s"><D:set><D:prop><C:%s>Changed through DAV</C:%s></D:prop></D:set></D:propertyupdate>`,
				test.descriptionNamespace, test.descriptionProperty, test.descriptionProperty)
			propertyUpdateResponse := davE2ERequest(client, "PROPPATCH", test.collectionURL, token,
				"application/xml", propertyUpdate, nil)
			davE2EExpect(t, propertyUpdateResponse, http.StatusMultiStatus)
			if !strings.Contains(propertyUpdateResponse.body, "200 OK") {
				t.Fatalf("DAV PROPPATCH did not update the property: %s", propertyUpdateResponse.body)
			}
			expectedDescription = "Changed through DAV"
			properties = davE2EExpect(t, davE2ERequest(client, "PROPFIND", test.collectionURL, token,
				"application/xml", propertyRequest, nil), http.StatusMultiStatus)
			if !strings.Contains(properties.body, expectedDescription) {
				t.Fatalf("PROPPATCH left an unexpected description: %s", properties.body)
			}
			objectURL := test.collectionURL + test.objectName
			if test.name == "calendar" {
				recurringURL := test.collectionURL + "recurring.ics"
				recurring := davE2EExpect(t, davE2ERequest(client, http.MethodPut, recurringURL, token,
					"text/calendar", davE2ERecurringCalendar, nil), http.StatusCreated)
				etag := recurring.header.Get("ETag")
				if etag == "" {
					t.Fatal("recurring object has no ETag")
				}
				query := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:getetag/><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"><C:time-range start="20261002T000000Z" end="20261003T000000Z"/></C:comp-filter></C:comp-filter></C:filter></C:calendar-query>`
				report := davE2EExpect(t, davE2ERequest(client, "REPORT", test.collectionURL, token,
					"application/xml", query, nil), http.StatusMultiStatus)
				if !strings.Contains(report.body, "recurring.ics") || !strings.Contains(report.body, "RRULE:FREQ=WEEKLY;COUNT=4") || !strings.Contains(report.body, html.EscapeString(etag)) {
					t.Fatalf("overlapping recurrence missing full data or ETag: %s", report.body)
				}
				davE2EExpect(t, davE2ERequest(client, "REPORT", test.collectionURL, otherToken,
					"application/xml", query, nil), http.StatusForbidden)
			}
			created := davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, test.contentType, test.first, nil), http.StatusCreated)
			firstETag := created.header.Get("ETag")
			if firstETag == "" {
				t.Fatal("created object has no ETag")
			}
			changed := davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, test.contentType, test.second,
				http.Header{"If-Match": {firstETag}}), http.StatusCreated)
			secondETag := changed.header.Get("ETag")
			if secondETag == "" || secondETag == firstETag {
				t.Fatalf("changed object ETag = %q; first = %q", secondETag, firstETag)
			}
			denied := davE2ERequest(client, http.MethodDelete, objectURL, otherToken, "", "",
				http.Header{"If-Match": {secondETag}})
			if denied.err != nil || (denied.status != http.StatusForbidden && denied.status != http.StatusNotFound) {
				t.Fatalf("other user deleted the object: %+v", denied)
			}
			davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, adminToken, "", "", nil), http.StatusOK)
			davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, test.contentType, test.first,
				http.Header{"If-Match": {firstETag}}), http.StatusPreconditionFailed)
			davE2EExpect(t, davE2ERequest(client, http.MethodDelete, objectURL, token, "", "",
				http.Header{"If-Match": {firstETag}}), http.StatusPreconditionFailed)
			davE2EExpect(t, davE2ERequest(client, http.MethodDelete, objectURL, token, "", "",
				http.Header{"If-None-Match": {secondETag}}), http.StatusPreconditionFailed)
			current := davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusOK)
			if !strings.Contains(current.body, "Second") {
				t.Fatalf("stale request changed the object: %s", current.body)
			}
			davE2EExpect(t, davE2ERequest(client, http.MethodDelete, objectURL, token, "", "",
				http.Header{"If-Match": {secondETag}}), http.StatusNoContent)
			davE2EExpect(t, davE2ERequest(client, http.MethodDelete, objectURL, token, "", "",
				http.Header{"If-Match": {secondETag}}), http.StatusPreconditionFailed)
			davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusNotFound)
			davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token, test.contentType, test.first,
				http.Header{"If-Match": {secondETag}}), http.StatusPreconditionFailed)

			createURL := test.collectionURL + "concurrent." + strings.TrimPrefix(test.objectName[strings.LastIndex(test.objectName, "."):], ".")
			results := make([]davE2EResponse, 2)
			var workers sync.WaitGroup
			for index, body := range []string{test.first, test.second} {
				workers.Add(1)
				go func(index int, body string) {
					defer workers.Done()
					results[index] = davE2ERequest(client, http.MethodPut, createURL, token, test.contentType, body,
						http.Header{"If-None-Match": {"*"}})
				}(index, body)
			}
			workers.Wait()
			for index := range results {
				if results[index].err != nil {
					t.Fatal(results[index].err)
				}
			}
			if !((results[0].status == http.StatusCreated && results[1].status == http.StatusPreconditionFailed) ||
				(results[1].status == http.StatusCreated && results[0].status == http.StatusPreconditionFailed)) {
				t.Fatalf("concurrent creates = %d (%s), %d (%s)", results[0].status, results[0].body, results[1].status, results[1].body)
			}

			baseline := davE2EExpect(t, davE2ERequest(client, http.MethodPut, createURL, token, test.contentType, test.first, nil), http.StatusCreated)
			baselineETag := baseline.header.Get("ETag")
			third := strings.Replace(test.first, "First", "Third", 1)
			updates := []string{test.second, third}
			for index, body := range updates {
				workers.Add(1)
				go func(index int, body string) {
					defer workers.Done()
					results[index] = davE2ERequest(client, http.MethodPut, createURL, token, test.contentType, body,
						http.Header{"If-Match": {baselineETag}})
				}(index, body)
			}
			workers.Wait()
			for index := range results {
				if results[index].err != nil {
					t.Fatal(results[index].err)
				}
			}
			if !((results[0].status == http.StatusCreated && results[1].status == http.StatusPreconditionFailed) ||
				(results[1].status == http.StatusCreated && results[0].status == http.StatusPreconditionFailed)) {
				t.Fatalf("concurrent updates = %d (%s), %d (%s)", results[0].status, results[0].body, results[1].status, results[1].body)
			}
			current = davE2EExpect(t, davE2ERequest(client, http.MethodGet, createURL, token, "", "", nil), http.StatusOK)
			winner := "Second"
			if results[1].status == http.StatusCreated {
				winner = "Third"
			}
			if !strings.Contains(current.body, winner) {
				t.Fatalf("concurrent winner %q missing from object: %s", winner, current.body)
			}

			fourth := strings.Replace(test.first, "First", "Fourth", 1)
			currentETag := current.header.Get("ETag")
			if currentETag == "" {
				t.Fatal("current object has no ETag")
			}
			workers.Add(2)
			go func() {
				defer workers.Done()
				results[0] = davE2ERequest(client, http.MethodPut, createURL, token, test.contentType, fourth,
					http.Header{"If-Match": {currentETag}})
			}()
			go func() {
				defer workers.Done()
				results[1] = davE2ERequest(client, http.MethodDelete, createURL, token, "", "",
					http.Header{"If-Match": {currentETag}})
			}()
			workers.Wait()
			if results[0].err != nil || results[1].err != nil {
				t.Fatalf("concurrent PUT/DELETE errors: %v, %v", results[0].err, results[1].err)
			}
			if !((results[0].status == http.StatusCreated && results[1].status == http.StatusPreconditionFailed) ||
				(results[1].status == http.StatusNoContent && results[0].status == http.StatusPreconditionFailed)) {
				t.Fatalf("concurrent PUT/DELETE = %d (%s), %d (%s)", results[0].status, results[0].body, results[1].status, results[1].body)
			}
			if results[0].status == http.StatusCreated {
				current = davE2EExpect(t, davE2ERequest(client, http.MethodGet, createURL, token, "", "", nil), http.StatusOK)
				if !strings.Contains(current.body, "Fourth") {
					t.Fatalf("successful PUT content missing: %s", current.body)
				}
			} else {
				davE2EExpect(t, davE2ERequest(client, http.MethodGet, createURL, token, "", "", nil), http.StatusNotFound)
			}
			davE2EExpect(t, davE2ERequest(client, http.MethodDelete, test.collectionURL, token, "", "",
				http.Header{"If-Match": {"\"stale-collection-etag\""}}), http.StatusNoContent)
			davE2EExpect(t, davE2ERequest(client, http.MethodGet, createURL, token, "", "", nil), http.StatusNotFound)
		})
	}
}
