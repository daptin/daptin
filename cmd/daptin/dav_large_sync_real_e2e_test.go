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

// This opt-in case exercises the boundary that previously rejected both
// collections at 10,001 resources. All objects are written through DAV.
func TestDAVLargeSyncRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_DAV_LARGE_E2E") != "1" {
		t.Skip("set DAPTIN_DAV_LARGE_E2E=1 for the 10,001-object DAV sync case")
	}
	runDAVLargeSyncRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-large-sync.db"))
}

func TestDAVLargeSyncPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	if os.Getenv("DAPTIN_DAV_LARGE_E2E") != "1" {
		t.Skip("set DAPTIN_DAV_LARGE_E2E=1 for the 10,001-object DAV sync case")
	}
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVLargeSyncRealE2E(t, "postgres", dsn)
}

func runDAVLargeSyncRealE2E(t *testing.T, databaseType, connectionString string) {
	t.Helper()
	usedPorts := make(map[int]bool)
	client := &http.Client{Timeout: 60 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString,
			olricPort: olricPort, schema: davE2EAccessSchema,
		})
		return base, process
	}
	firstURL, first := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
	first.stopProcess()
	base, process := start()
	defer process.stopProcess()
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-large-sync-owner")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing DAV principal: %s", principal.body)
	}
	calendarURL := base + "/caldav/" + owner[1] + "/calendars/large/"
	bookURL := base + "/carddav/" + owner[1] + "/addressbooks/large/"
	davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", calendarURL, ownerToken, "", "", nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, "MKCOL", bookURL, ownerToken, "", "", nil), http.StatusCreated)
	baseline := make(map[string]string)
	for _, collectionURL := range []string{calendarURL, bookURL} {
		props := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collectionURL, ownerToken,
			"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:sync-token/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
		match := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(props.body)
		if len(match) != 2 {
			t.Fatalf("missing starting sync token: %s", props.body)
		}
		baseline[collectionURL] = match[1]
	}
	for i := 0; i < 10001; i++ {
		name := fmt.Sprintf("object-%05d", i)
		calendar := strings.Replace(davE2ECalendarA, "UID:dav-condition-test", "UID:dav-large-"+name, 1)
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, calendarURL+name+".ics", ownerToken,
			"text/calendar", calendar, nil), http.StatusCreated)
		card := strings.Replace(davE2ECardA, "UID:dav-condition-test", "UID:dav-large-"+name, 1)
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, bookURL+name+".vcf", ownerToken,
			"text/vcard", card, nil), http.StatusCreated)
		if i > 0 && i%1000 == 0 {
			t.Logf("created %d calendar objects and contacts", i)
		}
	}
	for _, collectionURL := range []string{calendarURL, bookURL} {
		for _, start := range []string{"", baseline[collectionURL]} {
			token := start
			seen := make(map[string]bool, 10001)
			for page := 0; page < 12; page++ {
				body := fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:limit><D:nresults>1000</D:nresults></D:limit><D:prop><D:getetag/></D:prop></D:sync-collection>`, token)
				response := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, ownerToken,
					"application/xml", body, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
				for _, match := range regexp.MustCompile(`<href>([^<]+\.(?:ics|vcf))</href>`).FindAllStringSubmatch(response.body, -1) {
					if seen[match[1]] {
						t.Fatalf("sync repeated %s", match[1])
					}
					seen[match[1]] = true
				}
				match := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(response.body)
				if len(match) != 2 {
					t.Fatalf("sync page %d returned no token: %s", page, response.body)
				}
				token = match[1]
				if !strings.Contains(response.body, "number-of-matches-within-limits") {
					break
				}
			}
			if len(seen) != 10001 {
				t.Fatalf("%s synced %d of 10001 resources from token %t", collectionURL, len(seen), start != "")
			}
			t.Logf("synchronized all %d resources in %s from token %t", len(seen), collectionURL, start != "")
		}
		props := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collectionURL, ownerToken,
			"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:sync-token/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
		if !strings.Contains(props.body, "urn:daptin:dav-sync:") {
			t.Fatalf("large collection returned no sync-token property: %s", props.body)
		}
	}
}
