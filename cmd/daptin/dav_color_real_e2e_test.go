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

	"github.com/daptin/daptin/server/auth"
)

func TestDAVCalendarColorRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVCalendarColorRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-color.db"))
}

func TestDAVCalendarColorPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVCalendarColorRealE2E(t, "postgres", dsn)
}

func runDAVCalendarColorRealE2E(t *testing.T, databaseType, connectionString string) {
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
	defer func() { process.stopProcess() }()
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-color-owner")
	readerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-color-reader")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-color-other")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing calendar principal: %s", principal.body)
	}
	home := "/caldav/" + owner[1] + "/calendars/"
	plainURL := base + home + "plain/"
	colorURL := base + home + "colored/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", plainURL, ownerToken, "", "", nil), http.StatusCreated)
	mkcalendar := `<C:mkcalendar xmlns:C="urn:ietf:params:xml:ns:caldav" xmlns:A="http://apple.com/ns/ical/"><D:set xmlns:D="DAV:"><D:prop><A:calendar-color>#12abcf</A:calendar-color></D:prop></D:set></C:mkcalendar>`
	davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", colorURL, ownerToken, "application/xml", mkcalendar, nil), http.StatusCreated)
	badURL := base + home + "invalid-color/"
	davE2EExpect(t, davE2ERequest(client, "MKCALENDAR", badURL, ownerToken, "application/xml",
		strings.Replace(mkcalendar, "#12abcf", "red", 1), nil), http.StatusBadRequest)
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", badURL, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusNotFound)
	colorID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "colored")
	apiURL := base + "/api/collection/" + colorID
	getColor := func(token, name string) string {
		t.Helper()
		result := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/collection", token, nil, http.StatusOK)
		for _, item := range result.(map[string]interface{})["data"].([]interface{}) {
			attrs := item.(map[string]interface{})["attributes"].(map[string]interface{})
			if attrs["name"] == name {
				return attrs["color"].(string)
			}
		}
		t.Fatalf("calendar %q absent from JSON:API list: %#v", name, result)
		return ""
	}
	propfind := `<D:propfind xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/"><D:prop><A:calendar-color/></D:prop></D:propfind>`
	getDAVColor := func(requestURL, token, want string) {
		t.Helper()
		response := davE2EExpect(t, davE2ERequest(client, "PROPFIND", requestURL, token, "application/xml", propfind,
			http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
		if !strings.Contains(response.body, ">"+want+"<") {
			t.Fatalf("calendar color = %q, want %q: %s", response.body, want, response.body)
		}
	}
	if got := getColor(ownerToken, "plain"); got != "#3B82F6" {
		t.Fatalf("new calendar default color = %q", got)
	}
	getDAVColor(plainURL, ownerToken, "#3B82F6")
	if got := getColor(ownerToken, "colored"); got != "#12ABCF" {
		t.Fatalf("MKCALENDAR color = %q", got)
	}
	getDAVColor(colorURL, ownerToken, "#12ABCF")
	homeColors := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+home, ownerToken, "application/xml", propfind,
		http.Header{"Depth": {"1"}}), http.StatusMultiStatus)
	if !strings.Contains(homeColors.body, "#12ABCF") || !strings.Contains(homeColors.body, "#3B82F6") {
		t.Fatalf("calendar home did not list collection colors: %s", homeColors.body)
	}
	patch := func(value string) string {
		return `<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/"><D:set><D:prop><A:calendar-color>` + value + `</A:calendar-color></D:prop></D:set></D:propertyupdate>`
	}
	result := davE2EExpect(t, davE2ERequest(client, "PROPPATCH", colorURL, ownerToken,
		"application/xml", patch("#7654ab"), nil), http.StatusMultiStatus)
	if !strings.Contains(result.body, "200 OK") || getColor(ownerToken, "colored") != "#7654AB" {
		t.Fatalf("color PROPPATCH did not persist: %s", result.body)
	}
	invalid := davE2EExpect(t, davE2ERequest(client, "PROPPATCH", colorURL, ownerToken,
		"application/xml", patch("red"), nil), http.StatusMultiStatus)
	if !strings.Contains(invalid.body, "400 Bad Request") || getColor(ownerToken, "colored") != "#7654AB" {
		t.Fatalf("invalid DAV color changed collection: %s", invalid.body)
	}
	jsonPatch := func(color string) map[string]interface{} {
		return accessGroupsE2ERecordPayload("collection", colorID, map[string]interface{}{"color": color})
	}
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, apiURL, ownerToken, jsonPatch("#a1b2c3"), http.StatusOK)
	if got := getColor(ownerToken, "colored"); got != "#A1B2C3" {
		t.Fatalf("JSON:API color = %q", got)
	}
	deniedInvalid := davE2ERequest(client, http.MethodPatch, apiURL, ownerToken,
		"application/vnd.api+json", `{"data":{"type":"collection","id":"`+colorID+`","attributes":{"color":"blue"}}}`, nil)
	if deniedInvalid.err != nil || deniedInvalid.status != http.StatusBadRequest || getColor(ownerToken, "colored") != "#A1B2C3" {
		t.Fatalf("invalid JSON:API color changed collection: %+v", deniedInvalid)
	}
	groupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-color-readers")
	usersGroupID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "usergroup", "name", "users")
	readerID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-color-reader@test.local")
	accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/user_account/"+readerID+"/relationships/usergroup_id", adminToken,
		map[string]interface{}{"data": []map[string]string{{"type": "usergroup", "id": usersGroupID}, {"type": "usergroup", "id": groupID}}}, http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share", ownerToken, "application/json",
		fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"usergroup_id":%q,"permission":%d}}`, colorID, groupID, auth.GroupRead), nil), http.StatusOK)
	if got := getColor(readerToken, "colored"); got != "#A1B2C3" {
		t.Fatalf("read delegate color = %q", got)
	}
	getDAVColor(colorURL, readerToken, "#A1B2C3")
	discovery := davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/discover", readerToken,
		"application/json", `{"attributes":{}}`, nil), http.StatusOK)
	if !strings.Contains(discovery.body, colorID) || strings.Contains(discovery.body, "#A1B2C3") {
		t.Fatalf("collection discovery omitted the share or exposed color: %s", discovery.body)
	}
	if response := davE2ERequest(client, "PROPPATCH", colorURL, readerToken, "application/xml", patch("#111111"), nil); response.err != nil || strings.Contains(response.body, "200 OK") {
		t.Fatalf("read delegate changed color: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodPatch, apiURL, readerToken, "application/vnd.api+json",
		`{"data":{"type":"collection","id":"`+colorID+`","attributes":{"color":"#111111"}}}`, nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("read delegate changed color through JSON:API: %+v", response)
	}
	if response := davE2ERequest(client, "PROPFIND", colorURL, otherToken, "application/xml", propfind, nil); response.err != nil || strings.Contains(response.body, "#A1B2C3") {
		t.Fatalf("unshared account read color: %+v", response)
	}
	if got := getColor(ownerToken, "colored"); got != "#A1B2C3" {
		t.Fatalf("denied update changed color to %q", got)
	}
	copyURL := base + home + "copy/"
	davE2EExpect(t, davE2ERequest(client, "COPY", colorURL, ownerToken, "", "",
		http.Header{"Destination": {copyURL}, "Depth": {"0"}}), http.StatusCreated)
	getDAVColor(copyURL, ownerToken, "#A1B2C3")
	remove := `<D:propertyupdate xmlns:D="DAV:" xmlns:A="http://apple.com/ns/ical/"><D:remove><D:prop><A:calendar-color/></D:prop></D:remove></D:propertyupdate>`
	davE2EExpect(t, davE2ERequest(client, "PROPPATCH", colorURL, ownerToken, "application/xml", remove, nil), http.StatusMultiStatus)
	getDAVColor(colorURL, ownerToken, "#3B82F6")
	process.stopProcess()
	base, process = start()
	colorURL = base + home + "colored/"
	copyURL = base + home + "copy/"
	apiURL = base + "/api/collection/" + colorID
	getDAVColor(colorURL, ownerToken, "#3B82F6")
	getDAVColor(copyURL, ownerToken, "#A1B2C3")
	if got := getColor(ownerToken, "colored"); got != "#3B82F6" {
		t.Fatalf("restarted collection color = %q", got)
	}
}
