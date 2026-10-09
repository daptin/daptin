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
)

func TestDAVCalendarPropertiesAndTransfersRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav-mutations.db")
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
	base, process := start()
	defer process.stopProcess()
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-mutation-owner")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-mutation-other")
	destinationReaderToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-mutation-destination-reader")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing owner principal: %s", principal.body)
	}
	home := "/caldav/" + owner[1] + "/calendars/"
	for _, name := range []string{"source", "target", "third", "fourth"} {
		davE2EExpect(t, davE2ERequest(client, "MKCOL", base+home+name+"/", ownerToken, "", "", nil), http.StatusCreated)
	}
	source := base + home + "source/"
	target := base + home + "target/"
	third := base + home + "third/"
	fourth := base + home + "fourth/"
	patch := `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Visible name</D:displayname><C:calendar-description>Shared planning</C:calendar-description></D:prop></D:set></D:propertyupdate>`
	result := davE2EExpect(t, davE2ERequest(client, "PROPPATCH", source, ownerToken, "application/xml", patch, nil), http.StatusMultiStatus)
	if !strings.Contains(result.body, "200 OK") {
		t.Fatalf("calendar PROPPATCH failed: %s", result.body)
	}
	properties := davE2EExpect(t, davE2ERequest(client, "PROPFIND", source, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><D:displayname/><C:calendar-description/></D:prop></D:propfind>`, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	if !strings.Contains(properties.body, "Visible name") || !strings.Contains(properties.body, "Shared planning") {
		t.Fatalf("calendar metadata was not stored: %s", properties.body)
	}
	invalid := `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:set><D:prop><D:displayname>Must not commit</D:displayname><D:resourcetype/></D:prop></D:set></D:propertyupdate>`
	rejected := davE2EExpect(t, davE2ERequest(client, "PROPPATCH", source, ownerToken, "application/xml", invalid, nil), http.StatusMultiStatus)
	if !strings.Contains(rejected.body, "403 Forbidden") || !strings.Contains(rejected.body, "424 Failed Dependency") {
		t.Fatalf("protected property did not reject the atomic patch: %s", rejected.body)
	}
	properties = davE2EExpect(t, davE2ERequest(client, "PROPFIND", source, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	if !strings.Contains(properties.body, "Visible name") || strings.Contains(properties.body, "Must not commit") {
		t.Fatalf("failed PROPPATCH changed calendar metadata: %s", properties.body)
	}
	removeDisplayName := `<D:propertyupdate xmlns:D="DAV:"><D:remove><D:prop><D:displayname/></D:prop></D:remove></D:propertyupdate>`
	davE2EExpect(t, davE2ERequest(client, "PROPPATCH", source, ownerToken, "application/xml", removeDisplayName, nil), http.StatusMultiStatus)
	properties = davE2EExpect(t, davE2ERequest(client, "PROPFIND", source, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, http.Header{"Depth": {"0"}}), http.StatusMultiStatus)
	if !strings.Contains(properties.body, ">source<") {
		t.Fatalf("removing the display name did not restore the URL name: %s", properties.body)
	}
	if response := davE2ERequest(client, "PROPPATCH", source, otherToken, "application/xml", patch, nil); response.err != nil || response.status == http.StatusOK || strings.Contains(response.body, "200 OK") {
		t.Fatalf("other account changed calendar metadata: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, source+"event.ics", ownerToken, "text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	transfer := func(method, from, to string, overwrite bool) davE2EResponse {
		header := http.Header{"Destination": {to}}
		if !overwrite {
			header.Set("Overwrite", "F")
		}
		return davE2ERequest(client, method, from, ownerToken, "", "", header)
	}
	davE2EExpect(t, transfer("COPY", source+"event.ics", source+"duplicate.ics", false), http.StatusConflict)
	davE2EExpect(t, transfer("COPY", source+"event.ics", target+"copy.ics", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, target+"copy.ics", ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, transfer("COPY", source+"event.ics", target+"copy.ics", false), http.StatusPreconditionFailed)
	davE2EExpect(t, transfer("COPY", source+"event.ics", target+"copy.ics", true), http.StatusNoContent)
	davE2EExpect(t, transfer("MOVE", target+"copy.ics", target+"renamed.ics", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, target+"copy.ics", ownerToken, "", "", nil), http.StatusNotFound)
	otherEvent := strings.Replace(davE2ECalendarA, "UID:dav-condition-test", "UID:dav-overwritten", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, target+"overwrite.ics", ownerToken, "text/calendar", otherEvent, nil), http.StatusCreated)
	davE2EExpect(t, transfer("MOVE", target+"renamed.ics", target+"overwrite.ics", true), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, target+"renamed.ics", ownerToken, "", "", nil), http.StatusNotFound)
	davE2EExpect(t, transfer("MOVE", target+"overwrite.ics", third+"moved.ics", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, third+"moved.ics", ownerToken, "", "", nil), http.StatusOK)
	if response := davE2ERequest(client, "COPY", source+"event.ics", otherToken, "", "", http.Header{"Destination": {third + "denied.ics"}}); response.err != nil || response.status == http.StatusCreated {
		t.Fatalf("other account copied a private calendar event: %+v", response)
	}
	groupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-mutation-share")
	destinationGroupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-mutation-destination-share")
	usersGroupID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "usergroup", "name", "users")
	otherID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-mutation-other@test.local")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch, base+"/api/user_account/"+otherID+"/relationships/usergroup_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersGroupID, groupID), nil), http.StatusNoContent)
	destinationReaderID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-mutation-destination-reader@test.local")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch, base+"/api/user_account/"+destinationReaderID+"/relationships/usergroup_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersGroupID, destinationGroupID), nil), http.StatusNoContent)
	collectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "source")
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share", ownerToken, "application/json",
		fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"usergroup_id":%q,"permission":%d}}`, collectionID, groupID, auth.GroupRead), nil), http.StatusOK)
	destinationCollectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "fourth")
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share", ownerToken, "application/json",
		fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"usergroup_id":%q,"permission":%d}}`, destinationCollectionID, destinationGroupID, auth.GroupRead), nil), http.StatusOK)
	eventID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath", home+"source/event.ics")
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, source+"event.ics", otherToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, transfer("MOVE", source+"event.ics", fourth+"relocated.ics", false), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, fourth+"relocated.ics", ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, fourth+"relocated.ics", destinationReaderToken, "", "", nil), http.StatusOK)
	movedRow := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/calendar/"+eventID, ownerToken, nil, http.StatusOK)
	if !strings.Contains(fmt.Sprint(movedRow), home+"fourth/relocated.ics") {
		t.Fatalf("MOVE did not preserve the event resource identity: %#v", movedRow)
	}
	if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+eventID, otherToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("old calendar share leaked through moved event: %+v", response)
	}

	locked := base + home + "locked/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", locked, ownerToken, "", "", nil), http.StatusCreated)
	lockedID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "locked")
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share", ownerToken, "application/json",
		fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"usergroup_id":%q,"permission":%d}}`,
			lockedID, groupID, auth.GroupRead|auth.GroupCreate|auth.GroupRefer|auth.GroupUpdate), nil), http.StatusOK)
	delegateEvent := strings.Replace(davE2ECalendarA, "UID:dav-condition-test", "UID:dav-delegate-owned-delete", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, locked+"delegate.ics", otherToken,
		"text/calendar", delegateEvent, nil), http.StatusCreated)
	if denied := davE2ERequest(client, http.MethodDelete, locked, ownerToken, "", "", nil); denied.err != nil || denied.status == http.StatusNoContent {
		t.Fatalf("calendar deletion bypassed a child event's row permissions: %+v", denied)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, locked+"delegate.ics", otherToken, "", "", nil), http.StatusOK)
}
