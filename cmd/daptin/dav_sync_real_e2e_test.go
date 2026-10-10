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

func TestDAVCollectionTransferAndSyncRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVCollectionTransferAndSyncRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-sync.db"))
}

func TestDAVCollectionTransferAndSyncPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVCollectionTransferAndSyncRealE2E(t, "postgres", dsn)
}

func runDAVCollectionTransferAndSyncRealE2E(t *testing.T, databaseType, connectionString string) {
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
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-sync-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-sync-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing calendar owner: %s", principal.body)
	}
	home := base + "/caldav/" + owner[1] + "/calendars/"
	source := home + "source/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", source, ownerToken, "", "", nil), http.StatusCreated)
	event := davE2ECalendarA
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, source+"one.ics", ownerToken, "text/calendar", event, nil), http.StatusCreated)
	zero := strings.Replace(event, "UID:dav-condition-test", "UID:dav-sync-zero", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, source+"zero.ics", ownerToken, "text/calendar", zero, nil), http.StatusCreated)
	sourceID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "source")
	transfer := func(method, from, to, depth string, overwrite bool) davE2EResponse {
		header := http.Header{"Destination": {to}}
		if depth != "" {
			header.Set("Depth", depth)
		}
		if !overwrite {
			header.Set("Overwrite", "F")
		}
		return davE2ERequest(client, method, from, ownerToken, "", "", header)
	}
	shallow := home + "shallow/"
	davE2EExpect(t, transfer("COPY", source, shallow, "0", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, shallow+"one.ics", ownerToken, "", "", nil), http.StatusNotFound)
	copyPath := home + "copy/"
	davE2EExpect(t, transfer("COPY", source, copyPath, "", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, copyPath+"one.ics", ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, transfer("COPY", source, copyPath, "", false), http.StatusPreconditionFailed)
	if denied := davE2ERequest(client, "COPY", source, otherToken, "", "", http.Header{"Destination": {home + "denied/"}}); denied.err != nil || denied.status == http.StatusCreated {
		t.Fatalf("unrelated user copied a private calendar: %+v", denied)
	}
	moved := home + "moved/"
	davE2EExpect(t, transfer("MOVE", source, moved, "", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, source+"one.ics", ownerToken, "", "", nil), http.StatusNotFound)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, moved+"one.ics", ownerToken, "", "", nil), http.StatusOK)
	movedID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "moved")
	if movedID != sourceID {
		t.Fatalf("calendar MOVE changed collection identity: before=%s after=%s", sourceID, movedID)
	}
	propfind := `<D:propfind xmlns:D="DAV:"><D:prop><D:sync-token/><D:supported-report-set/></D:prop></D:propfind>`
	properties := davE2EExpect(t, davE2ERequest(client, "PROPFIND", moved, ownerToken,
		"application/xml", propfind, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	if !strings.Contains(properties.body, "sync-collection") || !strings.Contains(properties.body, "urn:daptin:dav-sync:") {
		t.Fatalf("calendar sync is not discoverable: %s", properties.body)
	}
	syncReport := func(token string, limit int) davE2EResponse {
		body := fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:limit><D:nresults>%d</D:nresults></D:limit><D:prop><D:getetag/></D:prop></D:sync-collection>`, token, limit)
		return davE2ERequest(client, "REPORT", moved, ownerToken, "application/xml", body, nil)
	}
	initial := davE2EExpect(t, syncReport("", 1), http.StatusMultiStatus)
	if !strings.Contains(initial.body, "one.ics") || !strings.Contains(initial.body, "507 Insufficient Storage") {
		t.Fatalf("initial sync did not provide a partial page: %s", initial.body)
	}
	token := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(initial.body)
	if len(token) != 2 {
		t.Fatalf("initial sync returned no token: %s", initial.body)
	}
	continued := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(continued.body, "zero.ics") || strings.Contains(continued.body, "507 Insufficient Storage") {
		t.Fatalf("initial sync did not finish on its continuation page: %s", continued.body)
	}
	token = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(continued.body)
	if len(token) != 2 {
		t.Fatalf("continued sync returned no token: %s", continued.body)
	}
	beforeTransient := token[1]
	for _, name := range []string{"flash-one", "flash-two"} {
		transient := strings.Replace(event, "UID:dav-condition-test", "UID:dav-sync-"+name, 1)
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, moved+name+".ics", ownerToken,
			"text/calendar", transient, nil), http.StatusCreated)
		davE2EExpect(t, davE2ERequest(client, http.MethodDelete, moved+name+".ics", ownerToken,
			"", "", nil), http.StatusNoContent)
	}
	transientPage := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(transientPage.body, "flash-one.ics") ||
		!strings.Contains(transientPage.body, "404 Not Found") ||
		!strings.Contains(transientPage.body, "507 Insufficient Storage") {
		t.Fatalf("sync lost the first transient deletion: %s", transientPage.body)
	}
	token = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(transientPage.body)
	if len(token) != 2 || token[1] == beforeTransient {
		t.Fatalf("transient mutation did not advance the sync token: %s", transientPage.body)
	}
	transientEnd := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(transientEnd.body, "flash-two.ics") ||
		strings.Contains(transientEnd.body, "flash-one.ics") ||
		strings.Contains(transientEnd.body, "507 Insufficient Storage") {
		t.Fatalf("sync did not finish transient deletions: %s", transientEnd.body)
	}
	token = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(transientEnd.body)
	if len(token) != 2 {
		t.Fatalf("transient continuation returned no token: %s", transientEnd.body)
	}
	// Replacing a member at the same href with identical content still changes
	// its mapping and must be reported even though the ETag returns to its old value.
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, moved+"zero.ics", ownerToken,
		"", "", nil), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, moved+"zero.ics", ownerToken,
		"text/calendar", zero, nil), http.StatusCreated)
	replaced := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(replaced.body, "zero.ics") || strings.Contains(replaced.body, "404 Not Found") {
		t.Fatalf("sync lost the recreated member: %s", replaced.body)
	}
	token = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(replaced.body)
	if len(token) != 2 {
		t.Fatalf("recreated member returned no token: %s", replaced.body)
	}
	second := strings.Replace(event, "UID:dav-condition-test", "UID:dav-sync-second", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, moved+"two.ics", ownerToken, "text/calendar", second, nil), http.StatusCreated)
	changed := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(changed.body, "two.ics") || strings.Contains(changed.body, "one.ics") {
		t.Fatalf("incremental sync did not isolate the new event: %s", changed.body)
	}
	token = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(changed.body)
	if len(token) != 2 {
		t.Fatalf("incremental sync returned no token: %s", changed.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, moved+"two.ics", ownerToken, "", "", nil), http.StatusNoContent)
	removed := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(removed.body, "two.ics") || !strings.Contains(removed.body, "404 Not Found") {
		t.Fatalf("sync did not report the deleted event: %s", removed.body)
	}
	token = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(removed.body)
	if len(token) != 2 {
		t.Fatalf("deletion sync returned no token: %s", removed.body)
	}
	oneID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+owner[1]+"/calendars/moved/one.ics")
	jsonDelete := davE2ERequest(client, http.MethodDelete, base+"/api/calendar/"+oneID, ownerToken, "", "", nil)
	if jsonDelete.err != nil || jsonDelete.status/100 != 2 {
		t.Fatalf("calendar JSON:API delete failed: %+v", jsonDelete)
	}
	jsonRemoved := davE2EExpect(t, syncReport(token[1], 1), http.StatusMultiStatus)
	if !strings.Contains(jsonRemoved.body, "one.ics") || !strings.Contains(jsonRemoved.body, "404 Not Found") ||
		strings.Contains(jsonRemoved.body, "zero.ics") {
		t.Fatalf("sync did not report the JSON:API deletion: %s", jsonRemoved.body)
	}
	invalid := davE2EExpect(t, syncReport("urn:daptin:dav-sync:invalid", 1), http.StatusForbidden)
	if !strings.Contains(invalid.body, "valid-sync-token") {
		t.Fatalf("invalid token omitted DAV error: %s", invalid.body)
	}
	// Overwrite deletes the old collection and its checkpoints, then copies
	// the source calendar's current membership rather than merging children.
	davE2EExpect(t, transfer("COPY", copyPath, moved, "", true), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, moved+"one.ics", ownerToken, "", "", nil), http.StatusOK)
	if stale := davE2ERequest(client, "REPORT", moved, ownerToken, "application/xml",
		fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, token[1]), nil); stale.err != nil || stale.status != http.StatusForbidden {
		t.Fatalf("overwritten calendar accepted its old token: %+v", stale)
	}
}
