package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin/server/auth"
)

const davE2EPrivateCalendar = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:dav-private-test\r\nDTSTAMP:20261005T120000Z\r\nDTSTART:20261006T140000Z\r\nDTEND:20261006T150000Z\r\nCLASS:PRIVATE\r\nSUMMARY:Private appointment\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

// Event owners in this schema receive access through a private group link,
// rather than an irrevocable UserRead bit on every event they create.
var davE2EShareSchema = fmt.Sprintf(`Tables:
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
  - TableName: usergroup
    Permission: %d
    DefaultPermission: %d
`, auth.GroupPeek, auth.GroupCRUD, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD|auth.GroupExecute,
	auth.GuestRefer, auth.GuestRefer)

// The administrator chooses both table access and the rights attached to new
// collection and event rows. DAV only consumes those ordinary resource grants.
func TestDAVConfiguredGroupDefaultsRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, grant := range []struct {
		name       string
		permission auth.AuthPermission
		canRead    bool
		canWrite   bool
	}{
		{name: "read", permission: auth.GroupRead, canRead: true},
		{name: "peek", permission: auth.GroupPeek},
		{name: "edit", permission: auth.GroupCRUD, canRead: true, canWrite: true},
	} {
		t.Run(grant.name, func(t *testing.T) {
			usedPorts := make(map[int]bool)
			databasePath := filepath.Join(t.TempDir(), "dav-group-defaults.db")
			schema := fmt.Sprintf(`Tables:
  - TableName: collection
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
  - TableName: calendar
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
`, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD, grant.permission,
				auth.UserCRUD, auth.GroupCRUD, grant.permission)
			start := func() (string, *transportE2EDaptinProcess) {
				port := freeTransportE2EPort(t, usedPorts)
				httpsPort := freeTransportE2EPort(t, usedPorts)
				olricPort := freeTransportE2EPortPair(t, usedPorts)
				base := fmt.Sprintf("http://127.0.0.1:%d", port)
				process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
					databaseType: "sqlite3", connectionString: databasePath, olricPort: olricPort, schema: schema,
				})
				return base, process
			}
			client := &http.Client{Timeout: 20 * time.Second}
			firstURL, first := start()
			adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
			ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
			first.stopProcess()
			base, second := start()
			defer second.stopProcess()

			ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "calendar-owner-"+grant.name)
			delegateToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "calendar-delegate-"+grant.name)
			principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
				"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
			match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
			if len(match) != 2 {
				t.Fatalf("owner principal reference ID missing: %s", principal.body)
			}
			collectionURL := base + "/caldav/" + match[1] + "/calendars/team/"
			eventURL := collectionURL + "event.ics"
			davE2EExpect(t, davE2ERequest(client, "MKCOL", collectionURL, ownerToken, "", "", nil), http.StatusCreated)
			davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, ownerToken,
				"text/calendar", davE2ECalendarA, nil), http.StatusCreated)

			if grant.canRead {
				listing := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collectionURL, delegateToken,
					"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
				if !strings.Contains(listing.body, "team") {
					t.Fatalf("delegated calendar missing from PROPFIND: %s", listing.body)
				}
				content := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil), http.StatusOK)
				if !strings.Contains(content.body, "SUMMARY:First") {
					t.Fatalf("delegate did not read the granted event: %s", content.body)
				}
				accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 1)
				if grant.canWrite {
					davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, delegateToken,
						"text/calendar", davE2ECalendarB, nil), http.StatusCreated)
					changed := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, ownerToken, "", "", nil), http.StatusOK)
					if !strings.Contains(changed.body, "SUMMARY:Second") {
						t.Fatalf("owner did not see delegate's edit: %s", changed.body)
					}
					createdURL := collectionURL + "created-by-delegate.ics"
					davE2EExpect(t, davE2ERequest(client, http.MethodPut, createdURL, delegateToken,
						"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
					davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, ownerToken, "", "", nil), http.StatusOK)
					createdID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
						"/caldav/"+match[1]+"/calendars/team/created-by-delegate.ics")
					delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email",
						"calendar-delegate-edit@test.local")
					created := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/calendar/"+createdID,
						adminToken, nil, http.StatusOK)
					attributes := created.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
					if attributes["user_account_id"] != delegateID {
						t.Fatalf("delegate-created event owner = %v, want %s", attributes["user_account_id"], delegateID)
					}
				} else {
					response := davE2ERequest(client, http.MethodPut, eventURL, delegateToken,
						"text/calendar", davE2ECalendarB, nil)
					if response.err != nil || response.status != http.StatusForbidden {
						t.Fatalf("read-only delegate changed an event: %+v", response)
					}
				}
			} else {
				response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil)
				if response.err != nil || response.status == http.StatusOK {
					t.Fatalf("peek-only delegate read event content: %+v", response)
				}
				accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 0)
				freeBusy := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
					"application/xml", `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20261006T000000Z" end="20261007T000000Z"/></C:free-busy-query>`,
					http.Header{"Depth": {"1"}}), http.StatusOK)
				if !strings.Contains(freeBusy.body, "20261006T120000Z/20261006T130000Z") ||
					strings.Contains(freeBusy.body, "SUMMARY") {
					t.Fatalf("peek-only free/busy returned wrong data: %s", freeBusy.body)
				}

				privateURL := collectionURL + "private.ics"
				davE2EExpect(t, davE2ERequest(client, http.MethodPut, privateURL, ownerToken,
					"text/calendar", davE2EPrivateCalendar, nil), http.StatusCreated)
				readersID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-detail-readers")
				usersID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "usergroup", "name", "users")
				delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "calendar-delegate-peek@test.local")
				davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
					base+"/api/user_account/"+delegateID+"/relationships/usergroup_id", adminToken,
					"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersID, readersID), nil), http.StatusNoContent)
				collectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "team")
				publicID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath", "/caldav/"+match[1]+"/calendars/team/event.ics")
				for _, link := range []string{
					base + "/api/collection/" + collectionID + "/relationships/usergroup_id",
					base + "/api/calendar/" + publicID + "/relationships/usergroup_id",
				} {
					davE2EExpect(t, davE2ERequest(client, http.MethodPatch, link, adminToken,
						"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersID, readersID), nil), http.StatusNoContent)
				}
				davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil), http.StatusOK)
				if response := davE2ERequest(client, http.MethodGet, privateURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
					t.Fatalf("delegate read private content: %+v", response)
				}
				accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 1)
				freeBusy = davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
					"application/xml", `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20261006T000000Z" end="20261007T000000Z"/></C:free-busy-query>`,
					http.Header{"Depth": {"1"}}), http.StatusOK)
				if !strings.Contains(freeBusy.body, "20261006T140000Z/20261006T150000Z") || strings.Contains(freeBusy.body, "Private appointment") {
					t.Fatalf("private event free/busy was not isolated: %s", freeBusy.body)
				}
			}
		})
	}
}

func TestDAVSharedThroughOrdinaryRelationshipsRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVSharedThroughOrdinaryRelationshipsRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-relationships.db"))
}

func TestDAVSharedThroughOrdinaryRelationshipsPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVSharedThroughOrdinaryRelationshipsRealE2E(t, "postgres", dsn)
}

func runDAVSharedThroughOrdinaryRelationshipsRealE2E(t *testing.T, databaseType, connectionString string) {
	t.Helper()
	usedPorts := make(map[int]bool)
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: davE2EShareSchema,
		})
		return base, process
	}
	client := &http.Client{Timeout: 20 * time.Second}
	firstURL, first := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
	first.stopProcess()
	base, second := start()
	defer second.stopProcess()

	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-share-owner")
	delegateToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-share-delegate")
	unrelatedToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-share-unrelated")
	ownerGroupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-share-owner-private")
	groupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-share-team")
	usersGroupID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "usergroup", "name", "users")
	ownerID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-share-owner@test.local")
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch, base+"/api/user_account/"+ownerID+"/relationships/usergroup_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersGroupID, ownerGroupID), nil), http.StatusNoContent)
	delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-share-delegate@test.local")
	membershipURL := base + "/api/user_account/" + delegateID + "/relationships/usergroup_id"
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch, membershipURL, adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersGroupID, groupID), nil), http.StatusNoContent)

	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("owner principal reference ID missing: %s", principal.body)
	}
	collectionURL := base + "/caldav/" + match[1] + "/calendars/team/"
	eventURL := collectionURL + "event.ics"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", collectionURL, ownerToken, "", "", nil), http.StatusCreated)
	collectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "team")
	shareURL := base + "/action/collection/share"
	shareGroup := func(group string, grant auth.AuthPermission, token string) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, shareURL, token, "application/json",
			fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"usergroup_id":%q,"permission":%d}}`, collectionID, group, grant), nil)
	}
	davE2EExpect(t, shareGroup(ownerGroupID, auth.GroupCRUD, adminToken), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, ownerToken,
		"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, ownerToken, "", "", nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.status == http.StatusOK || response.err != nil {
		t.Fatalf("delegate read before grant: %+v", response)
	}
	eventID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath", "/caldav/"+match[1]+"/calendars/team/event.ics")
	davE2EExpect(t, shareGroup(groupID, auth.GroupRead, adminToken), http.StatusOK)
	content := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil), http.StatusOK)
	if !strings.Contains(content.body, "SUMMARY:First") {
		t.Fatalf("delegate did not read the related event: %s", content.body)
	}
	visibleCollections := transportE2EGetJSON(t, client, base+"/api/collection", delegateToken)
	listed := accessGroupsE2EDataArray(t, visibleCollections)
	if len(listed) != 1 {
		t.Fatalf("delegate collection discovery = %#v", visibleCollections)
	}
	listedCollection := listed[0].(map[string]interface{})
	listedAttributes := listedCollection["attributes"].(map[string]interface{})
	if listedCollection["id"] != collectionID || listedAttributes["user_account_id"] != match[1] {
		t.Fatalf("shared collection lost its canonical owner: %#v", listedCollection)
	}
	accessGroupsE2EAssertListCount(t, client, base, unrelatedToken, "collection", 0)
	if response := davE2ERequest(client, http.MethodGet, eventURL, unrelatedToken, "", "", nil); response.status == http.StatusOK || response.err != nil {
		t.Fatalf("unrelated user read the shared event: %+v", response)
	}
	accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 1)

	// Removing the event relationship must revoke both DAV and JSON:API reads.
	eventLink := base + "/api/calendar/" + eventID + "/relationships/usergroup_id"
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, eventLink, adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q}]}`, groupID), nil), http.StatusNoContent)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.status == http.StatusOK || response.err != nil {
		t.Fatalf("delegate read after revocation: %+v", response)
	}
	privileges := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collectionURL, delegateToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-privilege-set/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	decoder := xml.NewDecoder(strings.NewReader(privileges.body))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode DAV privilege response: %v", err)
		}
		if start, ok := token.(xml.StartElement); ok && start.Name.Space == "DAV:" && start.Name.Local == "unbind" {
			t.Fatalf("collection advertised unbind after event-row revocation: %s", privileges.body)
		}
	}
	accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 0)

	share := func(permission auth.AuthPermission, token string) davE2EResponse {
		return shareGroup(groupID, permission, token)
	}
	if denied := share(auth.GroupRead, unrelatedToken); denied.err != nil || denied.status == http.StatusOK {
		t.Fatalf("unrelated user changed the calendar share: %+v", denied)
	}
	davE2EExpect(t, share(auth.UserRead, ownerToken), http.StatusBadRequest)
	groupURL := base + "/api/usergroup/" + groupID
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, groupURL, adminToken,
		accessGroupsE2ERecordPayload("usergroup", groupID, map[string]interface{}{"permission": 0}), http.StatusOK)
	davE2EExpect(t, share(auth.GroupRead, ownerToken), http.StatusForbidden)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("denied usergroup reference changed event access: %+v", response)
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, groupURL, adminToken,
		accessGroupsE2ERecordPayload("usergroup", groupID, map[string]interface{}{"permission": int64(auth.GuestRefer)}), http.StatusOK)
	davE2EExpect(t, share(auth.GroupRead, adminToken), http.StatusOK)
	readable := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil), http.StatusOK)
	if readable.header.Get("ETag") == "" {
		t.Fatal("event GET did not provide an ETag")
	}
	usage := transportE2EGetJSON(t, client, base+"/api/api_usage?page%5Bsize%5D=100", adminToken)
	usageRows := accessGroupsE2EDataArray(t, usage)
	meteredDelegateRead := false
	for _, item := range usageRows {
		attributes := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attributes["entity_type"] == "calendar" && attributes["method"] == http.MethodGet &&
			attributes["endpoint"] == "/caldav/"+match[1]+"/calendars/team/event.ics" &&
			attributes["user_account_id"] == delegateID && attributes["state"] == "completed" {
			meteredDelegateRead = true
		}
	}
	if !meteredDelegateRead {
		t.Fatalf("delegated DAV read was not metered to the active account: %#v", usage)
	}
	query := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"/></C:comp-filter></C:filter></C:calendar-query>`
	queryResult := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", query, nil), http.StatusMultiStatus)
	if !strings.Contains(queryResult.body, "SUMMARY:First") {
		t.Fatalf("delegated calendar-query omitted the event: %s", queryResult.body)
	}
	multiget := fmt.Sprintf(`<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop><D:href>%s</D:href></C:calendar-multiget>`,
		"/caldav/"+match[1]+"/calendars/team/event.ics")
	multigetResult := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", multiget, nil), http.StatusMultiStatus)
	if !strings.Contains(multigetResult.body, "SUMMARY:First") {
		t.Fatalf("delegated calendar-multiget omitted the event: %s", multigetResult.body)
	}
	accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 1)
	davE2EExpect(t, share(0, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("delegate retained event access after calendar share revocation: %+v", response)
	}
	accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 0)
	if response := davE2ERequest(client, http.MethodPut, eventURL, delegateToken, "text/calendar", davE2ECalendarB,
		http.Header{"If-Match": {readable.header.Get("ETag")}}); response.err != nil || response.status == http.StatusOK || response.status == http.StatusCreated {
		t.Fatalf("revoked delegate wrote with a previously valid ETag: %+v", response)
	}

	davE2EExpect(t, share(auth.GroupPeek, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("peek-only delegate read event details: %+v", response)
	}
	busy := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20261006T000000Z" end="20261007T000000Z"/></C:free-busy-query>`,
		http.Header{"Depth": {"1"}}), http.StatusOK)
	if !strings.Contains(busy.body, "20261006T120000Z/20261006T130000Z") || strings.Contains(busy.body, "SUMMARY") {
		t.Fatalf("peek-only share exposed event details: %s", busy.body)
	}

	davE2EExpect(t, share(auth.GroupCRUD, ownerToken), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, delegateToken,
		"text/calendar", davE2ECalendarB, nil), http.StatusCreated)
	changed := davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, ownerToken, "", "", nil), http.StatusOK)
	if !strings.Contains(changed.body, "SUMMARY:Second") {
		t.Fatalf("owner did not see delegate edit: %s", changed.body)
	}
	createdURL := collectionURL + "delegate-created.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, createdURL, delegateToken,
		"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	createdReadable := davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, delegateToken, "", "", nil), http.StatusOK)
	if createdReadable.header.Get("ETag") == "" {
		t.Fatal("delegate-created event GET did not provide an ETag")
	}
	createdID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+match[1]+"/calendars/team/delegate-created.ics")
	created := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/calendar/"+createdID,
		adminToken, nil, http.StatusOK)
	createdAttributes := created.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if createdAttributes["user_account_id"] != delegateID {
		t.Fatalf("delegate-created event owner = %v, want %s", createdAttributes["user_account_id"], delegateID)
	}
	createdUsage := transportE2EGetJSON(t, client, base+"/api/api_usage?page%5Bsize%5D=100", adminToken)
	meteredDelegateCreate := false
	for _, item := range accessGroupsE2EDataArray(t, createdUsage) {
		attributes := item.(map[string]interface{})["attributes"].(map[string]interface{})
		if attributes["entity_type"] == "calendar" && attributes["method"] == http.MethodPost &&
			attributes["endpoint"] == "/caldav/"+match[1]+"/calendars/team/delegate-created.ics" &&
			attributes["user_account_id"] == delegateID && attributes["state"] == "completed" {
			meteredDelegateCreate = true
		}
	}
	if !meteredDelegateCreate {
		t.Fatal("delegate-created event was not metered to the active account")
	}
	davE2EExpect(t, share(0, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, createdURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("delegate retained created event after calendar share revocation: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+createdID, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("delegate retained direct event access after calendar share revocation: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodPut, createdURL, delegateToken, "text/calendar", davE2ECalendarB,
		http.Header{"If-Match": {createdReadable.header.Get("ETag")}}); response.err != nil || response.status == http.StatusOK || response.status == http.StatusCreated {
		t.Fatalf("revoked creator wrote with a previously valid ETag: %+v", response)
	}
	directURL := base + "/api/calendar/" + createdID
	if response := davE2ERequest(client, http.MethodPatch, directURL, delegateToken, "application/vnd.api+json",
		fmt.Sprintf(`{"data":{"type":"calendar","id":%q,"attributes":{"rpath":%q}}}`, createdID,
			"/caldav/"+match[1]+"/calendars/team/delegate-created.ics"), nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("revoked creator updated the event through JSON:API: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodDelete, directURL, delegateToken, "", "", nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("revoked creator deleted the event through JSON:API: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, directURL, ownerToken, "", "", nil), http.StatusOK)

	davE2EExpect(t, share(auth.GroupExecute, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("manage-only delegate read event details: %+v", response)
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, groupURL, adminToken,
		accessGroupsE2ERecordPayload("usergroup", groupID, map[string]interface{}{"permission": 0}), http.StatusOK)
	davE2EExpect(t, share(0, delegateToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("delegate retained access after managing their own share: %+v", response)
	}
}
