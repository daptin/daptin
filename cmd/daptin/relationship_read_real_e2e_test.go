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

func TestRelationshipReadRealE2E(t *testing.T) {
	requireRealE2E(t)
	runRelationshipReadRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "relationships.db"))
}

func TestRelationshipReadPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runRelationshipReadRealE2E(t, "postgres", dsn)
}

func runRelationshipReadRealE2E(t *testing.T, databaseType, connectionString string) {
	schema := fmt.Sprintf(`Tables:
  - TableName: collection
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: bookable
    Permission: %d
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
  - TableName: user_account
    AccessGroups:
      - Name: users
        Permission: %d
    DefaultGroups:
      - Name: users
        Permission: %d
  - TableName: usergroup
    Permission: %d
    DefaultPermission: %d
`, auth.UserCRUD|auth.UserExecute, auth.GroupCRUD,
		auth.GuestPeek|auth.GuestRead|auth.GuestExecute|auth.UserCRUD, auth.GuestPeek|auth.GuestRead|auth.UserCRUD|auth.UserExecute, auth.GroupCRUD, auth.GroupRead,
		auth.GroupRefer, auth.GroupRefer, auth.GuestRefer, auth.GuestRefer)
	usedPorts := map[int]bool{}
	startServer := func() *daptinE2EFixture {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: schema,
		})
		return &daptinE2EFixture{URL: base, Client: &http.Client{Timeout: 30 * time.Second}, Process: process}
	}
	first := startServer()
	admin := first.SignupAdmin(t)
	ftpE2ESetConfig(t, first.Client, first.URL, admin, "caldav.enable", "true")
	first.Process.stopProcess()
	fixture := startServer()
	owner := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "relation-owner")
	other := accessGroupsE2ESignupSigninUser(t, fixture.Client, fixture.URL, admin, "relation-other")
	principal := davE2EExpect(t, davE2ERequest(fixture.Client, "PROPFIND", fixture.URL+"/caldav/", owner,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("owner principal missing: %s", principal.body)
	}
	calendarURL := fixture.URL + "/caldav/" + match[1] + "/calendars/"
	createCalendar := func(name string) string {
		t.Helper()
		davE2EExpect(t, davE2ERequest(fixture.Client, "MKCOL", calendarURL+name+"/", owner, "", "", nil), http.StatusCreated)
		return accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "collection", "name", name)
	}
	destination := createCalendar("relation-destination")
	bookable := accessGroupsE2ECreateRecord(t, fixture.Client, fixture.URL, owner, "bookable", map[string]interface{}{
		"title": "Relation check", "published": true, "destination_collection_id": destination,
		"time_zone": "UTC", "weekly_hours": `{}`, "duration_minutes": 30, "increment_minutes": 30,
		"capacity": 1, "horizon_days": 30,
	})
	base := fixture.URL + "/api/bookable/" + bookable
	link := base + "/relationships/collection_id"
	data := func(token, requestURL string) []interface{} {
		t.Helper()
		response := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, requestURL, token, nil, http.StatusOK)
		items, ok := response.(map[string]interface{})["data"].([]interface{})
		if !ok {
			t.Fatalf("relationship did not return an array: %#v", response)
		}
		return items
	}
	if len(data(owner, link)) != 0 {
		t.Fatal("empty relationship returned links")
	}
	plain := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, base, owner, nil, http.StatusOK)
	relationships := plain.(map[string]interface{})["data"].(map[string]interface{})["relationships"].(map[string]interface{})
	if member, ok := relationships["collection_id"].(map[string]interface{}); !ok {
		t.Fatalf("relationship member missing: %#v", relationships)
	} else if data, ok := member["data"].([]interface{}); !ok || len(data) != 0 {
		t.Fatalf("ordinary record linkage changed: %#v", member)
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, base+"?included_relations=collection_id", owner, nil, http.StatusOK)

	ids := map[string]bool{}
	for i := 0; i < 53; i++ {
		id := createCalendar(fmt.Sprintf("relation-%02d", i))
		ids[id] = true
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", owner,
			map[string]interface{}{"bookable_ref": bookable, "calendar_ref": id, "enabled": true}, http.StatusOK)
		if i == 0 && len(data(owner, link)) != 1 {
			t.Fatal("one linked calendar was not returned")
		}
	}
	firstPage := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		link+"?page%5Bnumber%5D=1&page%5Bsize%5D=10&sort=reference_id", owner, nil, http.StatusOK).(map[string]interface{})
	pageLinks := firstPage["links"].(map[string]interface{})
	if pageLinks["total"] != float64(53) || pageLinks["next_page_url"] == nil {
		t.Fatalf("relationship pagination did not expose the total and next page: %#v", firstPage)
	}
	ownerExpanded := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		base+"?included_relations=collection_id", owner, nil, http.StatusOK).(map[string]interface{})
	ownerLinks := ownerExpanded["data"].(map[string]interface{})["relationships"].(map[string]interface{})["collection_id"].(map[string]interface{})["data"].([]interface{})
	if len(ownerLinks) != 50 {
		t.Fatalf("explicit include returned %d linked calendars, want its bounded 50", len(ownerLinks))
	}
	seen := map[string]bool{}
	for page := 1; page <= 6; page++ {
		requestURL := fmt.Sprintf("%s?page%%5Bnumber%%5D=%d&page%%5Bsize%%5D=10&sort=reference_id", link, page)
		items := data(owner, requestURL)
		want := 10
		if page == 6 {
			want = 3
		}
		if len(items) != want {
			t.Fatalf("page %d returned %d links, want %d", page, len(items), want)
		}
		for _, item := range items {
			entry := item.(map[string]interface{})
			id := entry["id"].(string)
			if entry["type"] != "collection" || !ids[id] || seen[id] {
				t.Fatalf("page %d returned unexpected or duplicate link: %#v", page, entry)
			}
			seen[id] = true
		}
	}
	if len(seen) != len(ids) {
		t.Fatalf("pagination lost links: got %d, want %d", len(seen), len(ids))
	}
	offsetPage := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		link+"?page%5Boffset%5D=50&page%5Blimit%5D=10&sort=reference_id", owner, nil, http.StatusOK).(map[string]interface{})
	if got := len(offsetPage["data"].([]interface{})); got != 3 {
		t.Fatalf("offset page returned %d links, want 3", got)
	}
	offsetLinks := offsetPage["links"].(map[string]interface{})
	if previous, ok := offsetLinks["prev_page_url"].(string); !ok || !strings.HasPrefix(previous, "/api/") {
		t.Fatalf("offset pagination produced an invalid previous URL: %#v", offsetLinks)
	}
	if len(accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		base+"/collection_id?page%5Bnumber%5D=6&page%5Bsize%5D=10&sort=reference_id", owner, nil, http.StatusOK))) != 3 {
		t.Fatal("related-resource route disagreed with linkage page")
	}
	if len(data(other, link)) != 0 {
		t.Fatal("shared parent exposed private linked calendars")
	}
	if len(accessGroupsE2EDataArray(t, accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		base+"/collection_id", other, nil, http.StatusOK))) != 0 {
		t.Fatal("related-resource route exposed private linked calendars")
	}
	withInclude := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		base+"?included_relations=collection_id", other, nil, http.StatusOK)
	for id := range ids {
		if strings.Contains(fmt.Sprint(withInclude), id) {
			t.Fatalf("included relationship exposed private calendar %s", id)
		}
	}
	for id := range ids {
		accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPost, fixture.URL+"/action/bookable/set_calendar", owner,
			map[string]interface{}{"bookable_ref": bookable, "calendar_ref": id, "enabled": false}, http.StatusOK)
		delete(ids, id)
		break
	}
	if len(data(owner, link+"?page%5Bsize%5D=100")) != 52 {
		t.Fatal("revoked link remained visible")
	}
	revokedPage := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		link+"?page%5Bnumber%5D=1&page%5Bsize%5D=10", owner, nil, http.StatusOK).(map[string]interface{})
	if revokedPage["links"].(map[string]interface{})["total"] != float64(52) {
		t.Fatalf("revoked link remained in pagination count: %#v", revokedPage["links"])
	}
	toOne := accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet, base+"/relationships/destination_collection_id", owner, nil, http.StatusOK)
	if toOne.(map[string]interface{})["data"].(map[string]interface{})["id"] != destination {
		t.Fatalf("to-one relationship regressed: %#v", toOne)
	}
	groupLinks := data(admin, fixture.URL+"/api/user_account/mine/relationships/usergroup_id")
	groupIDs := map[string]bool{}
	for _, item := range groupLinks {
		groupIDs[item.(map[string]interface{})["id"].(string)] = true
	}
	for _, name := range []string{"users", "administrators"} {
		id := accessGroupsE2EFindResourceID(t, fixture.Client, fixture.URL, admin, "usergroup", "name", name)
		if !groupIDs[id] {
			t.Fatalf("mine linkage returned a join ID instead of %s: %#v", name, groupLinks)
		}
	}
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodGet,
		fixture.URL+"/api/user_account/mine/usergroup_id?page%5Bnumber%5D=1&page%5Bsize%5D=10", admin, nil, http.StatusOK)
	accessGroupsE2ERequestJSON(t, fixture.Client, http.MethodPatch, base, admin,
		accessGroupsE2ERecordPayload("bookable", bookable, map[string]interface{}{"permission": int64(auth.UserCRUD | auth.UserExecute)}), http.StatusOK)
	for _, path := range []string{link, base + "/collection_id"} {
		if response := davE2ERequest(fixture.Client, http.MethodGet, path, other, "", "", nil); response.err != nil || response.status != http.StatusForbidden {
			t.Fatalf("private parent exposed linked calendar through %s: %+v", path, response)
		}
	}
}
