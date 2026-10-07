package main

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

type davE2EHrefResponse struct {
	Href      string `xml:"DAV: href"`
	PropStats []struct {
		Prop struct {
			DisplayName string `xml:"DAV: displayname"`
		} `xml:"DAV: prop"`
	} `xml:"DAV: propstat"`
}

func davE2ECollectionHref(t *testing.T, body, name string) string {
	t.Helper()
	var multiStatus struct {
		Responses []davE2EHrefResponse `xml:"DAV: response"`
	}
	if err := xml.Unmarshal([]byte(body), &multiStatus); err != nil {
		t.Fatal(err)
	}
	for _, response := range multiStatus.Responses {
		for _, propStat := range response.PropStats {
			if propStat.Prop.DisplayName == name {
				return response.Href
			}
		}
	}
	t.Fatalf("collection %q has no DAV href: %s", name, body)
	return ""
}

func TestDAVCollectionHrefsRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav-hrefs.db")
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
	token := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-href-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-href-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", token,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("principal reference ID missing: %s", principal.body)
	}

	for _, protocol := range []struct {
		prefix, home, objectName, contentType, content, report string
	}{
		{"caldav", "calendars", "event.ics", "text/calendar", davE2ECalendarA,
			`<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:getetag/><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"/></C:comp-filter></C:filter></C:calendar-query>`},
		{"carddav", "addressbooks", "person.vcf", "text/vcard", davE2ECardA,
			`<C:addressbook-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:getetag/><C:address-data/></D:prop><C:filter><C:prop-filter name="FN"/></C:filter></C:addressbook-query>`},
	} {
		for _, name := range []string{"Calendar 1", "100%", "日本語"} {
			t.Run(protocol.prefix+"/"+name, func(t *testing.T) {
				home := fmt.Sprintf("/%s/%s/%s/", protocol.prefix, match[1], protocol.home)
				collectionPath := home + url.PathEscape(name) + "/"
				davE2EExpect(t, davE2ERequest(client, "MKCOL", base+collectionPath, token, "", "", nil), http.StatusCreated)
				propfind := `<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/><D:displayname/></D:prop></D:propfind>`
				listing := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+home, token,
					"application/xml", propfind, http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
				href := davE2ECollectionHref(t, listing.body, name)
				if href != collectionPath {
					t.Fatalf("advertised href = %q, want %q", href, collectionPath)
				}
				collection := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+href, token,
					"application/xml", propfind, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
				if got := davE2ECollectionHref(t, collection.body, name); got != href {
					t.Fatalf("direct collection href = %q, want %q", got, href)
				}
				objectURL := base + href + protocol.objectName
				created := davE2EExpect(t, davE2ERequest(client, http.MethodPut, objectURL, token,
					protocol.contentType, protocol.content, nil), http.StatusCreated)
				if created.header.Get("ETag") == "" {
					t.Fatal("created object has no ETag")
				}
				got := davE2EExpect(t, davE2ERequest(client, http.MethodGet, objectURL, token, "", "", nil), http.StatusOK)
				if !strings.Contains(got.body, "UID:dav-condition-test") {
					t.Fatalf("GET through advertised href returned the wrong object: %s", got.body)
				}
				report := davE2EExpect(t, davE2ERequest(client, "REPORT", base+href, token,
					"application/xml", protocol.report, nil), http.StatusMultiStatus)
				if !strings.Contains(report.body, protocol.objectName) || !strings.Contains(report.body, "UID:dav-condition-test") {
					t.Fatalf("REPORT through advertised href did not return the object: %s", report.body)
				}
				davE2EExpect(t, davE2ERequest(client, "REPORT", base+href, otherToken,
					"application/xml", protocol.report, nil), http.StatusForbidden)
			})
		}
	}
}
