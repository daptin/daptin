package main

import (
	"encoding/json"
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

func TestDAVAccountShareRosterRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVAccountShareRosterRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-share-roster.db"))
}

func TestDAVAccountShareRosterPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVAccountShareRosterRealE2E(t, "postgres", dsn)
}

func runDAVAccountShareRosterRealE2E(t *testing.T, databaseType, connectionString string) {
	t.Helper()
	usedPorts := make(map[int]bool)
	client := &http.Client{Timeout: 30 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		return base, startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: davE2EShareSchema,
		})
	}
	base, process := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, base)
	ftpE2ESetConfig(t, client, base, adminToken, "caldav.enable", "true")
	process.stopProcess()
	base, process = start()
	defer func() { process.stopProcess() }()

	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "roster-owner")
	delegateToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "roster-delegate")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "roster-other")
	delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "roster-delegate@test.local")
	otherID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "roster-other@test.local")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	match := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(match) != 2 {
		t.Fatalf("owner principal missing: %s", principal.body)
	}
	for _, name := range []string{"roster", "other-roster"} {
		davE2EExpect(t, davE2ERequest(client, "MKCOL", base+"/caldav/"+match[1]+"/calendars/"+name+"/", ownerToken, "", "", nil), http.StatusCreated)
	}
	collectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "roster")
	otherCollectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "other-roster")
	list := func(token, calendar, cursor string) davE2EResponse {
		t.Helper()
		attrs := map[string]interface{}{"calendar_reference_id": calendar}
		if cursor != "" {
			attrs["cursor"] = cursor
		}
		body, err := json.Marshal(map[string]interface{}{"attributes": attrs})
		if err != nil {
			t.Fatal(err)
		}
		return davE2ERequest(client, http.MethodPost, base+"/action/collection/list_shares", token, "application/json", string(body), nil)
	}
	readShares := func(token string) []map[string]interface{} {
		t.Helper()
		response := davE2EExpect(t, list(token, collectionID, ""), http.StatusOK)
		var rows []struct {
			Attributes struct {
				CollectionID string                   `json:"collection_id"`
				Shares       []map[string]interface{} `json:"shares"`
				NextCursor   string                   `json:"next_cursor"`
			} `json:"Attributes"`
		}
		if err := json.Unmarshal([]byte(response.body), &rows); err != nil || len(rows) != 1 || rows[0].Attributes.CollectionID != collectionID || rows[0].Attributes.NextCursor != "" {
			t.Fatalf("invalid share roster: %v: %s", err, response.body)
		}
		if strings.Contains(response.body, "calshare:") || strings.Contains(response.body, "usergroup_id") {
			t.Fatalf("share roster leaked internal group data: %s", response.body)
		}
		return rows[0].Attributes.Shares
	}
	shareUser := func(account string, grant auth.AuthPermission) {
		t.Helper()
		davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", ownerToken,
			"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"user_account_id":%q,"permission":%d}}`, collectionID, account, grant), nil), http.StatusOK)
	}
	shareEmail := func(token, email string, grant auth.AuthPermission) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", token,
			"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"recipient_email":%q,"permission":%d}}`, collectionID, email, grant), nil)
	}
	assertOne := func(token, account string, grant auth.AuthPermission, busy, read, edit bool) {
		t.Helper()
		shares := readShares(token)
		if len(shares) != 1 {
			t.Fatalf("roster has %d shares, want one: %#v", len(shares), shares)
		}
		row := shares[0]
		if row["user_account_id"] != account || row["permission"] != float64(grant) || row["can_free_busy"] != busy || row["can_read"] != read || row["can_edit"] != edit {
			t.Fatalf("roster grant = %#v", row)
		}
		if row["name"] != "roster-delegate" || row["email"] != "roster-delegate@test.local" {
			t.Fatalf("roster account identity = %#v", row)
		}
	}
	if len(readShares(ownerToken)) != 0 {
		t.Fatal("new calendar already has direct account shares")
	}
	accounts := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/user_account?page%5Bsize%5D=200", ownerToken, nil, http.StatusOK)
	for _, item := range accessGroupsE2EDataArray(t, accounts) {
		if item.(map[string]interface{})["id"] == delegateID {
			t.Fatal("owner unexpectedly read the recipient account")
		}
	}
	ordinaryGroup := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "roster-ordinary-group")
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share", ownerToken,
		"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"usergroup_id":%q,"permission":%d}}`, collectionID, ordinaryGroup, auth.GroupRead), nil), http.StatusOK)
	if shares := readShares(ownerToken); len(shares) != 0 {
		t.Fatalf("ordinary group appeared in account roster: %#v", shares)
	}
	davE2EExpect(t, list(delegateToken, collectionID, ""), http.StatusForbidden)
	if response := shareEmail(otherToken, "roster-delegate@test.local", auth.GroupPeek); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("unrelated caller resolved a recipient: %+v", response)
	}
	if response := shareEmail(ownerToken, "missing@test.local", auth.GroupPeek); response.err != nil || response.status != http.StatusNotFound {
		t.Fatalf("missing recipient response: %+v", response)
	}
	if response := shareEmail(ownerToken, "ROSTER-DELEGATE@test.local", auth.GroupPeek); response.err != nil || response.status != http.StatusNotFound {
		t.Fatalf("non-exact recipient response: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", ownerToken,
		"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"user_account_id":%q,"recipient_email":"roster-delegate@test.local","permission":%d}}`, collectionID, delegateID, auth.GroupPeek), nil); response.err != nil || response.status != http.StatusBadRequest {
		t.Fatalf("two recipient selectors accepted: %+v", response)
	}
	if response := davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", ownerToken,
		"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"permission":%d}}`, collectionID, auth.GroupPeek), nil); response.err != nil || response.status != http.StatusBadRequest {
		t.Fatalf("missing recipient accepted: %+v", response)
	}
	shared := davE2EExpect(t, shareEmail(ownerToken, "roster-delegate@test.local", auth.GroupPeek), http.StatusOK)
	if !strings.Contains(shared.body, delegateID) {
		t.Fatalf("email share did not return recipient ID: %s", shared.body)
	}
	assertOne(ownerToken, delegateID, auth.GroupPeek, true, false, false)
	assertOne(adminToken, delegateID, auth.GroupPeek, true, false, false)
	accounts = accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/user_account?page%5Bsize%5D=200", ownerToken, nil, http.StatusOK)
	for _, item := range accessGroupsE2EDataArray(t, accounts) {
		if item.(map[string]interface{})["id"] == delegateID {
			t.Fatal("calendar share broadened account list access")
		}
	}
	otherCalendar := davE2EExpect(t, list(ownerToken, otherCollectionID, ""), http.StatusOK)
	if strings.Contains(otherCalendar.body, delegateID) {
		t.Fatalf("one calendar's share appeared in another: %s", otherCalendar.body)
	}
	davE2EExpect(t, list(delegateToken, collectionID, ""), http.StatusForbidden)
	davE2EExpect(t, list(otherToken, collectionID, ""), http.StatusForbidden)
	shareUser(delegateID, auth.GroupPeek|auth.GroupRead)
	assertOne(ownerToken, delegateID, auth.GroupPeek|auth.GroupRead, true, true, false)
	shareUser(delegateID, auth.GroupPeek|auth.GroupRead|auth.GroupUpdate)
	assertOne(ownerToken, delegateID, auth.GroupPeek|auth.GroupRead|auth.GroupUpdate, true, true, true)
	shareUser(otherID, auth.GroupRead)
	if shares := readShares(ownerToken); len(shares) != 2 {
		t.Fatalf("two account shares absent: %#v", shares)
	}
	shareUser(otherID, 0)
	assertOne(ownerToken, delegateID, auth.GroupPeek|auth.GroupRead|auth.GroupUpdate, true, true, true)
	if response := list(ownerToken, collectionID, "invalid"); response.err != nil || response.status != http.StatusBadRequest {
		t.Fatalf("invalid share cursor accepted: %+v", response)
	}
	if response := list(delegateToken, otherCollectionID, ""); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("delegate listed another calendar: %+v", response)
	}
	process.stopProcess()
	base, process = start()
	assertOne(ownerToken, delegateID, auth.GroupPeek|auth.GroupRead|auth.GroupUpdate, true, true, true)
	davE2EExpect(t, shareEmail(ownerToken, "roster-delegate@test.local", 0), http.StatusOK)
	if shares := readShares(ownerToken); len(shares) != 0 {
		t.Fatalf("revoked account remained in roster: %#v", shares)
	}
	listActionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "action", "action_name", "list_shares")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/action/"+listActionID, adminToken,
		accessGroupsE2ERecordPayload("action", listActionID, map[string]interface{}{"permission": 0}), http.StatusOK)
	if response := list(ownerToken, collectionID, ""); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("list shares bypassed its action permission: %+v", response)
	}
}
