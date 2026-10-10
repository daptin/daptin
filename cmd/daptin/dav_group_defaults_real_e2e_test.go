package main

import (
	"encoding/json"
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

const davE2EPrivateCalendar = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//Daptin//EN\r\nBEGIN:VEVENT\r\nUID:dav-private-test\r\nDTSTAMP:20261005T120000Z\r\nDTSTART:20261006T140000Z\r\nDTEND:20261006T150000Z\r\nCLASS:PRIVATE\r\nSUMMARY:Private appointment\r\nDESCRIPTION:Private description\r\nATTENDEE:mailto:private@example.invalid\r\nATTACH:https://example.invalid/private.pdf\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"

func davE2EContainsPrivateEventDetail(body string) bool {
	for _, detail := range []string{"Private appointment", "Private description", "private@example.invalid", "private.pdf"} {
		if strings.Contains(body, detail) {
			return true
		}
	}
	return false
}

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
  - TableName: user_account
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
`, auth.GroupPeek, auth.GroupCRUD, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD|auth.GroupExecute,
	auth.GuestRefer, auth.GuestRefer, auth.GroupRefer, auth.GroupRefer)

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
				if grant.name == "read" || grant.name == "edit" {
					mailServer := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_server", map[string]interface{}{
						"hostname": "localhost", "is_enabled": false, "listen_interface": "127.0.0.1:0",
						"max_size": 10000, "max_clients": 1, "xclient_on": false,
						"always_on_tls": false, "authentication_required": false,
					})
					mailAccount := accessGroupsE2ECreateRecord(t, client, base, adminToken, "mail_account", map[string]interface{}{
						"username": "calendar-" + grant.name + "@localhost", "password": "testpass123",
						"password_md5": "testpass123", "mail_server_id": mailServer,
					})
					collectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "team")
					davE2EExpect(t, davE2ERequest(client, http.MethodPatch,
						base+"/api/collection/"+collectionID+"/relationships/scheduling_mail_account_id", adminToken,
						"application/vnd.api+json", fmt.Sprintf(`{"data":{"type":"mail_account","id":"%s"}}`, mailAccount), nil), http.StatusNoContent)
					outboxURL := base + "/caldav/" + match[1] + "/schedule-outbox/"
					privileges := davE2ERequest(client, "PROPFIND", outboxURL, delegateToken,
						"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-privilege-set/></D:prop></D:propfind>`,
						http.Header{"Depth": {"0"}})
					if privileges.err != nil {
						t.Fatal(privileges.err)
					}
					if grant.canWrite {
						if privileges.status != http.StatusMultiStatus || !strings.Contains(privileges.body, "schedule-send-invite") ||
							!strings.Contains(privileges.body, "schedule-send-reply") || strings.Contains(privileges.body, "schedule-send-freebusy") {
							t.Fatalf("edit delegate has wrong outbox privileges: %+v", privileges)
						}
					} else if privileges.status != http.StatusForbidden {
						t.Fatalf("read-only delegate reached owner's scheduling outbox: %+v", privileges)
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
				privateQuery := `<C:calendar-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop><C:filter><C:comp-filter name="VCALENDAR"><C:comp-filter name="VEVENT"/></C:comp-filter></C:filter></C:calendar-query>`
				privateQueryResult := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
					"application/xml", privateQuery, nil), http.StatusMultiStatus)
				if davE2EContainsPrivateEventDetail(privateQueryResult.body) || !strings.Contains(privateQueryResult.body, "SUMMARY:First") {
					t.Fatalf("private calendar-query exposed details or omitted public event: %s", privateQueryResult.body)
				}
				privateMultiget := fmt.Sprintf(`<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop><D:href>%s</D:href></C:calendar-multiget>`,
					"/caldav/"+match[1]+"/calendars/team/private.ics")
				privateMultigetResult := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
					"application/xml", privateMultiget, nil), http.StatusMultiStatus)
				if davE2EContainsPrivateEventDetail(privateMultigetResult.body) {
					t.Fatalf("private calendar-multiget exposed details: %s", privateMultigetResult.body)
				}
				privateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
					"/caldav/"+match[1]+"/calendars/team/private.ics")
				if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+privateID, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
					t.Fatalf("delegate read private event through JSON:API: %+v", response)
				}
				accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 1)
				freeBusy = davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
					"application/xml", `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20261006T000000Z" end="20261007T000000Z"/></C:free-busy-query>`,
					http.Header{"Depth": {"1"}}), http.StatusOK)
				if !strings.Contains(freeBusy.body, "20261006T140000Z/20261006T150000Z") || davE2EContainsPrivateEventDetail(freeBusy.body) {
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
	capabilities := func(token string) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/collection/share_capabilities", token, "application/json",
			fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q}}`, collectionID), nil)
	}
	assertCapabilities := func(token string, group, account bool) {
		t.Helper()
		response := davE2EExpect(t, capabilities(token), http.StatusOK)
		var rows []struct {
			Attributes map[string]interface{} `json:"Attributes"`
		}
		if err := json.Unmarshal([]byte(response.body), &rows); err != nil || len(rows) != 1 {
			t.Fatalf("decode calendar capabilities: %v: %s", err, response.body)
		}
		groupValue, groupOK := rows[0].Attributes["can_share_group"].(bool)
		accountValue, accountOK := rows[0].Attributes["can_share_user"].(bool)
		if !groupOK || !accountOK || groupValue != group || accountValue != account {
			t.Fatalf("calendar capabilities = %#v, want group=%t account=%t", rows[0].Attributes, group, account)
		}
	}
	eventCapabilities := func(token, event string) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/collection/event_capabilities", token, "application/json",
			fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"event_reference_id":%q}}`, collectionID, event), nil)
	}
	assertEventCapabilities := func(token, event string, update, delete bool) {
		t.Helper()
		response := davE2EExpect(t, eventCapabilities(token, event), http.StatusOK)
		var rows []struct {
			Attributes map[string]interface{} `json:"Attributes"`
		}
		if err := json.Unmarshal([]byte(response.body), &rows); err != nil || len(rows) != 1 {
			t.Fatalf("decode event capabilities: %v: %s", err, response.body)
		}
		updateValue, updateOK := rows[0].Attributes["can_update"].(bool)
		deleteValue, deleteOK := rows[0].Attributes["can_delete"].(bool)
		if !updateOK || !deleteOK || updateValue != update || deleteValue != delete {
			t.Fatalf("event capabilities = %#v, want update=%t delete=%t", rows[0].Attributes, update, delete)
		}
	}
	davE2EExpect(t, shareGroup(ownerGroupID, auth.GroupCRUD, adminToken), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, ownerToken,
		"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, ownerToken, "", "", nil), http.StatusOK)
	groupCount := func() int {
		response := transportE2EGetJSON(t, client, base+"/api/usergroup?page%5Bsize%5D=100", adminToken)
		return len(accessGroupsE2EDataArray(t, response))
	}
	beforeCapabilities := groupCount()
	assertCapabilities(ownerToken, true, true)
	if response := capabilities(unrelatedToken); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("unrelated account queried calendar capabilities: %+v", response)
	}
	if response := capabilities(delegateToken); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("unshared delegate queried calendar capabilities: %+v", response)
	}
	if afterCapabilities := groupCount(); afterCapabilities != beforeCapabilities {
		t.Fatalf("capability queries created a usergroup: before=%d after=%d", beforeCapabilities, afterCapabilities)
	}
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.status == http.StatusOK || response.err != nil {
		t.Fatalf("delegate read before grant: %+v", response)
	}
	eventID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath", "/caldav/"+match[1]+"/calendars/team/event.ics")
	assertEventCapabilities(ownerToken, eventID, true, true)
	if response := davE2ERequest(client, http.MethodGet, eventURL, unrelatedToken, "", "", nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("ungranted direct event href did not return 403: %+v", response)
	}
	if response := davE2ERequest(client, "PROPFIND", collectionURL, unrelatedToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("ungranted direct collection href did not return 403: %+v", response)
	}
	if response := eventCapabilities(unrelatedToken, eventID); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("ungranted account queried event capabilities: %+v", response)
	}
	davE2EExpect(t, shareGroup(groupID, auth.GroupRead, adminToken), http.StatusOK)
	assertCapabilities(delegateToken, false, false)
	assertEventCapabilities(delegateToken, eventID, false, false)
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
	syncBody := `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`
	syncInitial := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", syncBody, nil), http.StatusMultiStatus)
	if !strings.Contains(syncInitial.body, "event.ics") {
		t.Fatalf("delegate sync omitted its readable event: %s", syncInitial.body)
	}
	syncToken := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(syncInitial.body)
	if len(syncToken) != 2 {
		t.Fatalf("delegate sync returned no token: %s", syncInitial.body)
	}
	transientURL := collectionURL + "transient.ics"
	transient := strings.Replace(davE2ECalendarA, "UID:dav-condition-test", "UID:dav-shared-transient", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, transientURL, ownerToken,
		"text/calendar", transient, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, transientURL, ownerToken,
		"", "", nil), http.StatusNoContent)
	syncBody = fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, syncToken[1])
	syncTransient := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", syncBody, nil), http.StatusMultiStatus)
	if !strings.Contains(syncTransient.body, "transient.ics") || !strings.Contains(syncTransient.body, "404 Not Found") {
		t.Fatalf("delegate sync lost a shared event created and deleted between reports: %s", syncTransient.body)
	}
	syncToken = regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(syncTransient.body)
	if len(syncToken) != 2 {
		t.Fatalf("delegate transient sync returned no token: %s", syncTransient.body)
	}

	// Removing the event relationship must revoke both DAV and JSON:API reads.
	eventLink := base + "/api/calendar/" + eventID + "/relationships/usergroup_id"
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, eventLink, adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q}]}`, groupID), nil), http.StatusNoContent)
	// Relationship edits through JSON:API bypass the DAV change log, but the
	// next direct read still enforces the revoked row grant.
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
	if response := eventCapabilities(delegateToken, eventID); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("revoked event link retained capability discovery: %+v", response)
	}

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
	if response := eventCapabilities(delegateToken, eventID); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("peek-only delegate queried event capabilities: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("peek-only delegate read event details: %+v", response)
	}
	busy := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20261006T000000Z" end="20261007T000000Z"/></C:free-busy-query>`,
		http.Header{"Depth": {"1"}}), http.StatusOK)
	if !strings.Contains(busy.body, "20261006T120000Z/20261006T130000Z") || strings.Contains(busy.body, "SUMMARY") {
		t.Fatalf("peek-only share exposed event details: %s", busy.body)
	}
	privateURL := collectionURL + "private.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, privateURL, ownerToken,
		"text/calendar", davE2EPrivateCalendar, nil), http.StatusCreated)
	privateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+match[1]+"/calendars/team/private.ics")
	if response := davE2ERequest(client, http.MethodGet, privateURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("peek-only share exposed private event through DAV: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+privateID, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("peek-only share exposed private event through JSON:API: %+v", response)
	}
	queryResult = davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", query, nil), http.StatusForbidden)
	if davE2EContainsPrivateEventDetail(queryResult.body) {
		t.Fatalf("peek-only calendar-query exposed private event details: %s", queryResult.body)
	}
	privateMultiget := fmt.Sprintf(`<C:calendar-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:caldav"><D:prop><C:calendar-data/></D:prop><D:href>%s</D:href></C:calendar-multiget>`,
		"/caldav/"+match[1]+"/calendars/team/private.ics")
	privateMultigetResult := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", privateMultiget, nil), http.StatusMultiStatus)
	if !strings.Contains(privateMultigetResult.body, "403 Forbidden") || davE2EContainsPrivateEventDetail(privateMultigetResult.body) {
		t.Fatalf("peek-only calendar-multiget exposed private event details: %s", privateMultigetResult.body)
	}
	busy = davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, delegateToken,
		"application/xml", `<C:free-busy-query xmlns:C="urn:ietf:params:xml:ns:caldav"><C:time-range start="20261006T000000Z" end="20261007T000000Z"/></C:free-busy-query>`,
		http.Header{"Depth": {"1"}}), http.StatusOK)
	if !strings.Contains(busy.body, "20261006T140000Z/20261006T150000Z") || davE2EContainsPrivateEventDetail(busy.body) {
		t.Fatalf("peek-only free/busy omitted private event or exposed details: %s", busy.body)
	}

	davE2EExpect(t, share(auth.GroupPeek|auth.GroupRead|auth.GroupUpdate, ownerToken), http.StatusOK)
	assertEventCapabilities(delegateToken, eventID, true, false)
	if response := davE2ERequest(client, http.MethodDelete, eventURL, delegateToken, "", "", nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("update-only delegate deleted event: %+v", response)
	}
	davE2EExpect(t, share(auth.GroupCRUD, ownerToken), http.StatusOK)
	assertEventCapabilities(delegateToken, eventID, true, true)
	deletableURL := collectionURL + "deletable.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, deletableURL, ownerToken,
		"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	deletableID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+match[1]+"/calendars/team/deletable.ics")
	assertEventCapabilities(delegateToken, deletableID, true, true)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, deletableURL, delegateToken, "", "", nil), http.StatusNoContent)
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
	assertCapabilities(delegateToken, true, true)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("manage-only delegate read event details: %+v", response)
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, groupURL, adminToken,
		accessGroupsE2ERecordPayload("usergroup", groupID, map[string]interface{}{"permission": 0}), http.StatusOK)
	davE2EExpect(t, share(0, delegateToken), http.StatusOK)
	if response := capabilities(delegateToken); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("revoked manager retained capability discovery: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("delegate retained access after managing their own share: %+v", response)
	}

	shareUser := func(account string, grant auth.AuthPermission, token string) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", token, "application/json",
			fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"user_account_id":%q,"permission":%d}}`, collectionID, account, grant), nil)
	}
	if response := shareUser(delegateID, auth.GroupRead, unrelatedToken); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("unrelated account created a user share: %+v", response)
	}
	userShare := davE2EExpect(t, shareUser(delegateID, auth.GroupRead, ownerToken), http.StatusOK)
	var userShareBody interface{}
	if err := json.Unmarshal([]byte(userShare.body), &userShareBody); err != nil {
		t.Fatalf("decode account share response: %v: %s", err, userShare.body)
	}
	userShareGroup, ok := accessGroupsE2EFindString(userShareBody, "usergroup_id")
	if !ok || userShareGroup == "" {
		t.Fatalf("account share did not return group handle: %s", userShare.body)
	}
	for i := 0; i < 2; i++ {
		repeated := davE2EExpect(t, shareUser(delegateID, auth.GroupRead, ownerToken), http.StatusOK)
		var repeatedBody interface{}
		if err := json.Unmarshal([]byte(repeated.body), &repeatedBody); err != nil {
			t.Fatalf("decode repeated share response: %v", err)
		}
		if id, _ := accessGroupsE2EFindString(repeatedBody, "usergroup_id"); id != userShareGroup {
			t.Fatalf("account share retry created another group: %s", repeated.body)
		}
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, unrelatedToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("account share exposed event to an unrelated account: %+v", response)
	}
	davE2EExpect(t, shareUser(delegateID, 0, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("account share revocation left event readable: %+v", response)
	}
	accessGroupsE2EAssertListCount(t, client, base, delegateToken, "calendar", 0)
	davE2EExpect(t, shareUser(delegateID, auth.GroupPeek, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("peek-only account share exposed event content: %+v", response)
	}
	davE2EExpect(t, shareUser(delegateID, auth.GroupCRUD, ownerToken), http.StatusOK)
	accountCreatedURL := collectionURL + "account-share-created.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, accountCreatedURL, delegateToken,
		"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, accountCreatedURL, ownerToken, "", "", nil), http.StatusOK)
	accountCreatedID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+match[1]+"/calendars/team/account-share-created.ics")
	davE2EExpect(t, shareUser(delegateID, 0, ownerToken), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("revoked account read its created event through DAV: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+accountCreatedID, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("revoked account read its created event through JSON:API: %+v", response)
	}
	aclOptions := davE2EExpect(t, davE2ERequest(client, http.MethodOptions, collectionURL, ownerToken, "", "", nil), http.StatusNoContent)
	if !strings.Contains(aclOptions.header.Get("Allow"), "ACL") {
		t.Fatalf("calendar did not advertise ACL: %v", aclOptions.header)
	}
	aclBody := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>%s/caldav/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace></D:acl>`, base, delegateID)
	davE2EExpect(t, davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", aclBody, nil), http.StatusOK)
	aclProps := davE2EExpect(t, davE2ERequest(client, "PROPFIND", collectionURL, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:acl/><D:owner/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(aclProps.body, "/caldav/"+delegateID+"/") || !strings.Contains(aclProps.body, "read") {
		t.Fatalf("calendar ACL did not show account grant: %s", aclProps.body)
	}
	aclRoundTrip := regexp.MustCompile(`(?s)<acl[^>]*>.*?</acl>`).FindString(aclProps.body)
	if aclRoundTrip == "" {
		t.Fatalf("calendar ACL property was not usable as an ACL request: %s", aclProps.body)
	}
	davE2EExpect(t, davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", aclRoundTrip, nil), http.StatusOK)
	aclPrincipalReport := davE2EExpect(t, davE2ERequest(client, "REPORT", collectionURL, ownerToken,
		"application/xml", `<D:acl-principal-prop-set xmlns:D="DAV:"><D:prop><D:displayname/><D:principal-URL/></D:prop></D:acl-principal-prop-set>`, nil), http.StatusMultiStatus)
	if !strings.Contains(aclPrincipalReport.body, "/caldav/"+delegateID+"/") || !strings.Contains(aclPrincipalReport.body, "dav-share-delegate") {
		t.Fatalf("calendar ACL principal report omitted the delegate: %s", aclPrincipalReport.body)
	}
	ownerGroupACL := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/caldav/groups/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace></D:acl>`, ownerGroupID)
	if response := davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", ownerGroupACL, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("calendar ACL changed a protected owner group: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+accountCreatedID, delegateToken, "", "", nil); response.err != nil || response.status != http.StatusOK {
		t.Fatalf("calendar ACL account grant did not allow JSON:API read: %+v", response)
	}
	if response := davE2ERequest(client, "ACL", collectionURL, unrelatedToken, "application/xml", `<D:acl xmlns:D="DAV:"/>`, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("unrelated account changed calendar ACL: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", `<D:acl xmlns:D="DAV:"/>`, nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("calendar ACL revocation left event readable: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+accountCreatedID, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("calendar ACL revocation left event readable through JSON:API: %+v", response)
	}
	groupPrincipal := "/caldav/groups/" + groupID + "/"
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, groupURL, adminToken,
		accessGroupsE2ERecordPayload("usergroup", groupID, map[string]interface{}{"permission": int64(auth.GuestRefer)}), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+groupPrincipal, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:resourcetype/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	groupACL := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>%s</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace></D:acl>`, groupPrincipal)
	davE2EExpect(t, davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", groupACL, nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil), http.StatusOK)
	deniedACL := strings.Replace(groupACL, "<D:grant>", "<D:deny>", 1)
	deniedACL = strings.Replace(deniedACL, "</D:grant>", "</D:deny>", 1)
	if response := davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", deniedACL, nil); response.err != nil || response.status < 400 {
		t.Fatalf("deny ACE unexpectedly replaced group ACL: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", `<D:acl xmlns:D="DAV:"/>`, nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("calendar group ACL revocation left event readable: %+v", response)
	}
	partialACL := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/caldav/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace><D:ace><D:principal><D:href>/caldav/groups/%s/</D:href></D:principal><D:grant><D:privilege><D:unsupported/></D:privilege></D:grant></D:ace></D:acl>`, delegateID, groupID)
	if response := davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", partialACL, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("invalid ACL did not fail atomically: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, accountCreatedURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("failed ACL left partial account access: %+v", response)
	}
	unrelatedID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-share-unrelated@test.local")
	davE2EExpect(t, shareUser(delegateID, auth.GroupExecute, ownerToken), http.StatusOK)
	assertCapabilities(delegateToken, true, true)
	davE2EExpect(t, shareUser(unrelatedID, auth.GroupRead, delegateToken), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, eventURL, unrelatedToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, shareUser(unrelatedID, 0, ownerToken), http.StatusOK)
	davE2EExpect(t, shareUser(delegateID, 0, ownerToken), http.StatusOK)
	collectionWorldID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "world", "table_name", "collection")
	collectionActionID := func(name string) string {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/action?page%5Bsize%5D=200", adminToken, nil, http.StatusOK)
		for _, item := range accessGroupsE2EDataArray(t, response) {
			row, _ := item.(map[string]interface{})
			attributes, _ := row["attributes"].(map[string]interface{})
			if attributes["action_name"] == name && attributes["world_id"] == collectionWorldID {
				if id, ok := row["id"].(string); ok {
					return id
				}
			}
		}
		t.Fatalf("collection action %s not found: %#v", name, response)
		return ""
	}
	shareUserActionID := collectionActionID("share_user")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/action/"+shareUserActionID, adminToken,
		accessGroupsE2ERecordPayload("action", shareUserActionID, map[string]interface{}{"permission": 0}), http.StatusOK)
	assertCapabilities(ownerToken, true, false)
	if response := shareUser(delegateID, auth.GroupRead, ownerToken); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("account sharing bypassed action permission: %+v", response)
	}
	if response := davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", aclBody, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("calendar ACL bypassed account sharing action permission: %+v", response)
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/action/"+shareUserActionID, adminToken,
		accessGroupsE2ERecordPayload("action", shareUserActionID, map[string]interface{}{"permission": int64(auth.AuthenticatedExecute)}), http.StatusOK)
	assertCapabilities(ownerToken, true, true)
	shareActionID := collectionActionID("share")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/action/"+shareActionID, adminToken,
		accessGroupsE2ERecordPayload("action", shareActionID, map[string]interface{}{"permission": 0}), http.StatusOK)
	assertCapabilities(ownerToken, false, true)
	if response := shareGroup(ownerGroupID, auth.GroupRead, ownerToken); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("group sharing bypassed action permission: %+v", response)
	}
	if response := davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", `<D:acl xmlns:D="DAV:"/>`, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("calendar ACL bypassed group sharing action permission: %+v", response)
	}
}
