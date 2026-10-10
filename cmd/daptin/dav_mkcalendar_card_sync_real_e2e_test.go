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

func TestDAVMkcalendarAndCardSyncRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVMkcalendarAndCardSyncRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-card-sync.db"))
}

func TestDAVMkcalendarAndCardSyncPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVMkcalendarAndCardSyncRealE2E(t, "postgres", dsn)
}

func runDAVMkcalendarAndCardSyncRealE2E(t *testing.T, databaseType, connectionString string) {
	t.Helper()
	usedPorts := make(map[int]bool)
	client := &http.Client{Timeout: 30 * time.Second}
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
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-card-sync-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-card-sync-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing DAV principal: %s", principal.body)
	}
	calendarURL := base + "/caldav/" + owner[1] + "/calendars/planning/"
	calendarHome := base + "/caldav/" + owner[1] + "/calendars/"
	options := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, calendarHome, ownerToken,
		"", "", nil), http.StatusNoContent)
	if !strings.Contains(options.header.Get("Allow"), "MKCALENDAR") {
		t.Fatalf("calendar home does not advertise MKCALENDAR: %v", options.header)
	}
	mkcalendar := `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:D="DAV:"><D:set><D:prop><D:displayname>Planning team</D:displayname><C:calendar-description>Shared plans</C:calendar-description></D:prop></D:set></C:mkcalendar>`
	created := davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", calendarURL, ownerToken,
		"application/xml", mkcalendar, nil), http.StatusCreated)
	if created.header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("MKCALENDAR did not disable caching: %v", created.header)
	}
	properties := davE2EExpect(t, davE2ERequest(client, "PROPFIND", calendarURL, ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:displayname/><C:calendar-description/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(properties.body, "Planning team") || !strings.Contains(properties.body, "Shared plans") {
		t.Fatalf("MKCALENDAR did not persist requested properties: %s", properties.body)
	}
	davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", calendarURL, ownerToken,
		"", "", nil), http.StatusMethodNotAllowed)
	emptyCalendarURL := calendarHome + "empty/"
	davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", emptyCalendarURL, ownerToken,
		"", "", nil), http.StatusCreated)
	denied := davE2ERequest(client, "MKCALENDAR", base+"/caldav/"+owner[1]+"/calendars/other/", otherToken,
		"", "", nil)
	if denied.err != nil || denied.status == http.StatusCreated {
		t.Fatalf("other user created calendar in owner's home: %+v", denied)
	}
	invalidURL := base + "/caldav/" + owner[1] + "/calendars/invalid/"
	invalid := strings.Replace(mkcalendar, "<D:displayname>Planning team</D:displayname>", "<C:calendar-timezone>bad</C:calendar-timezone>", 1)
	davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", invalidURL, ownerToken,
		"application/xml", invalid, nil), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", invalidURL, ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusNotFound)

	bookURL := base + "/carddav/" + owner[1] + "/addressbooks/people/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", bookURL, ownerToken, "", "", nil), http.StatusCreated)
	bookProps := davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:sync-token/><D:supported-report-set/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(bookProps.body, "sync-collection") || !strings.Contains(bookProps.body, "urn:daptin:dav-sync:") {
		t.Fatalf("CardDAV sync discovery missing: %s", bookProps.body)
	}
	syncReport := func(token string, limit int) davE2EResponse {
		body := fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:limit><D:nresults>%d</D:nresults></D:limit><D:prop><D:getetag/></D:prop></D:sync-collection>`, token, limit)
		return davE2ERequest(client, "REPORT", bookURL, ownerToken, "application/xml", body, http.Header{"Depth": {"0"}})
	}
	initial := davE2EExpect(t, syncReport("", 1), http.StatusMultiStatus)
	match := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(initial.body)
	if len(match) != 2 {
		t.Fatalf("CardDAV initial sync returned no token: %s", initial.body)
	}
	previousToken := match[1]
	for _, name := range []string{"one", "two"} {
		card := strings.Replace(davE2ECardA, "UID:dav-condition-test", "UID:dav-card-"+name, 1)
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, bookURL+name+".vcf", ownerToken,
			"text/vcard", card, nil), http.StatusCreated)
	}
	page := davE2EExpect(t, syncReport(previousToken, 1), http.StatusMultiStatus)
	if !strings.Contains(page.body, "one.vcf") || !strings.Contains(page.body, "507 Insufficient Storage") {
		t.Fatalf("CardDAV sync did not paginate: %s", page.body)
	}
	match = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(page.body)
	if len(match) != 2 {
		t.Fatalf("CardDAV page returned no token: %s", page.body)
	}
	end := davE2EExpect(t, syncReport(match[1], 1), http.StatusMultiStatus)
	if !strings.Contains(end.body, "two.vcf") || strings.Contains(end.body, "507 Insufficient Storage") {
		t.Fatalf("CardDAV sync continuation failed: %s", end.body)
	}
	match = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(end.body)
	if len(match) != 2 {
		t.Fatalf("CardDAV continuation returned no token: %s", end.body)
	}
	previousToken = match[1]
	updated := strings.Replace(davE2ECardA, "UID:dav-condition-test", "UID:dav-card-one", 1)
	updated = strings.Replace(updated, "FN:First", "FN:Updated First", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, bookURL+"one.vcf", ownerToken,
		"text/vcard", updated, nil), http.StatusCreated)
	updatedCard := davE2EExpect(t, davE2ERequest(client, http.MethodGet, bookURL+"one.vcf", ownerToken,
		"", "", nil), http.StatusOK)
	if !strings.Contains(updatedCard.body, "FN:Updated First") {
		t.Fatalf("CardDAV update did not persist: %s", updatedCard.body)
	}
	changed := davE2EExpect(t, syncReport(previousToken, 1), http.StatusMultiStatus)
	if !strings.Contains(changed.body, "one.vcf") || strings.Contains(changed.body, "507 Insufficient Storage") {
		t.Fatalf("CardDAV sync lost contact update: %s", changed.body)
	}
	match = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(changed.body)
	if len(match) != 2 {
		t.Fatalf("CardDAV update returned no token: %s", changed.body)
	}
	previousToken = match[1]
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL+"two.vcf", ownerToken,
		"", "", nil), http.StatusNoContent)
	removed := davE2EExpect(t, syncReport(previousToken, 1), http.StatusMultiStatus)
	if !strings.Contains(removed.body, "two.vcf") || !strings.Contains(removed.body, "404 Not Found") {
		t.Fatalf("CardDAV sync lost contact deletion: %s", removed.body)
	}
	match = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(removed.body)
	if len(match) != 2 {
		t.Fatalf("CardDAV deletion returned no token: %s", removed.body)
	}
	previousToken = match[1]
	transient := strings.Replace(davE2ECardA, "UID:dav-condition-test", "UID:dav-card-transient", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, bookURL+"transient.vcf", ownerToken,
		"text/vcard", transient, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL+"transient.vcf", ownerToken,
		"", "", nil), http.StatusNoContent)
	deleted := davE2EExpect(t, syncReport(previousToken, 1), http.StatusMultiStatus)
	if !strings.Contains(deleted.body, "transient.vcf") || !strings.Contains(deleted.body, "404 Not Found") {
		t.Fatalf("CardDAV sync lost transient deletion: %s", deleted.body)
	}
	if response := davE2ERequest(client, "REPORT", bookURL, otherToken, "application/xml",
		`<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, nil); response.err != nil || response.status == http.StatusMultiStatus {
		t.Fatalf("other user synced private address book: %+v", response)
	}
	secondBookURL := base + "/carddav/" + owner[1] + "/addressbooks/secondary/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", secondBookURL, ownerToken, "", "", nil), http.StatusCreated)
	wrongBookReport := fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, previousToken)
	davE2EExpect(t, davE2ERequest(client, "REPORT", secondBookURL, ownerToken,
		"application/xml", wrongBookReport, nil), http.StatusForbidden)
	davE2EExpect(t, syncReport("invalid", 1), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, secondBookURL, ownerToken, "", "", nil), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL, ownerToken, "", "", nil), http.StatusNoContent)
}
