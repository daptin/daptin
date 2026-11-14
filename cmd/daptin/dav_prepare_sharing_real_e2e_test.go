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

func TestDAVPrepareSharingRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVPrepareSharingRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-prepare.db"))
}

func TestDAVPrepareSharingPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVPrepareSharingRealE2E(t, "postgres", dsn)
}

func runDAVPrepareSharingRealE2E(t *testing.T, databaseType, connectionString string) {
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
	defer process.stopProcess()

	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "prepare-owner")
	delegateToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "prepare-delegate")
	readerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "prepare-reader")
	otherToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "prepare-other")
	delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "prepare-delegate@test.local")
	readerID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "prepare-reader@test.local")
	ownerID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "prepare-owner@test.local")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("owner principal missing: %s", principal.body)
	}
	calendarURL := base + "/caldav/" + owner[1] + "/calendars/prepared/"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", calendarURL, ownerToken, "", "", nil), http.StatusCreated)
	collectionID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "collection", "name", "prepared")
	call := func(action, token string) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/collection/"+action, token,
			"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q}}`, collectionID), nil)
	}
	if response := call("prepare_sharing", otherToken); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("unrelated user prepared calendar: %+v", response)
	}
	before := davE2EExpect(t, call("share_capabilities", ownerToken), http.StatusOK)
	if !strings.Contains(before.body, `"revocable_edit_sharing":false`) {
		t.Fatalf("new calendar incorrectly ready: %s", before.body)
	}

	// An older event retains its creator rights until the action migrates it.
	legacyURL := calendarURL + "legacy.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, legacyURL, adminToken, "text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	legacyID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+owner[1]+"/calendars/prepared/legacy.ics")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/calendar/"+legacyID, adminToken,
		accessGroupsE2ERecordPayload("calendar", legacyID, map[string]interface{}{"permission": int64(auth.UserCRUD | auth.GuestRead)}), http.StatusOK)
	groupsBefore := len(accessGroupsE2EDataArray(t, transportE2EGetJSON(t, client, base+"/api/usergroup?page%5Bsize%5D=100", adminToken)))
	if response := call("prepare_sharing", ownerToken); response.err != nil || response.status != http.StatusConflict {
		t.Fatalf("public event was accepted for revocable sharing: %+v", response)
	}
	groupsAfter := len(accessGroupsE2EDataArray(t, transportE2EGetJSON(t, client, base+"/api/usergroup?page%5Bsize%5D=100", adminToken)))
	if groupsAfter != groupsBefore {
		t.Fatalf("failed preparation committed an owner group: before=%d after=%d", groupsBefore, groupsAfter)
	}
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/calendar/"+legacyID, adminToken,
		accessGroupsE2ERecordPayload("calendar", legacyID, map[string]interface{}{"permission": int64(auth.UserCRUD)}), http.StatusOK)
	prepared := davE2EExpect(t, call("prepare_sharing", ownerToken), http.StatusOK)
	if !strings.Contains(prepared.body, `"events_updated":1`) {
		t.Fatalf("existing event was not migrated: %s", prepared.body)
	}
	ownerGroup := regexp.MustCompile(`"owner_group_id":"([0-9a-f-]{36})"`).FindStringSubmatch(prepared.body)
	if len(ownerGroup) != 2 {
		t.Fatalf("preparation returned no owner group: %s", prepared.body)
	}
	davE2EExpect(t, call("prepare_sharing", ownerToken), http.StatusOK)
	ready := davE2EExpect(t, call("share_capabilities", ownerToken), http.StatusOK)
	if !strings.Contains(ready.body, `"revocable_edit_sharing":true`) || !strings.Contains(ready.body, `"can_prepare_sharing":true`) {
		t.Fatalf("prepared calendar not reported ready: %s", ready.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, legacyURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+legacyID, ownerToken, "", "", nil), http.StatusOK)

	share := func(accountID string, grant auth.AuthPermission) {
		t.Helper()
		davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", ownerToken,
			"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"user_account_id":%q,"permission":%d}}`, collectionID, accountID, grant), nil), http.StatusOK)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPost, base+"/action/collection/share_user", ownerToken,
		"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,"recipient_email":"prepare-delegate@test.local","permission":%d}}`, collectionID, auth.GroupCRUD), nil), http.StatusOK)
	share(readerID, auth.GroupRead|auth.GroupPeek)
	createdURL := calendarURL + "delegate.ics"
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, createdURL, delegateToken, "text/calendar", davE2ECalendarB, nil), http.StatusCreated)
	createdID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+owner[1]+"/calendars/prepared/delegate.ics")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/calendar/"+createdID, adminToken,
		accessGroupsE2ERecordPayload("calendar", createdID, map[string]interface{}{"permission": int64(auth.UserCRUD)}), http.StatusOK)
	if response := call("share_capabilities", ownerToken); response.err != nil || !strings.Contains(response.body, `"revocable_edit_sharing":false`) {
		t.Fatalf("creator-owned event did not make policy unready: %+v", response)
	}
	migrated := davE2EExpect(t, call("prepare_sharing", ownerToken), http.StatusOK)
	if !strings.Contains(migrated.body, `"events_updated":1`) {
		t.Fatalf("delegate-created event was not migrated: %s", migrated.body)
	}
	roster := davE2EExpect(t, call("list_shares", ownerToken), http.StatusOK)
	if strings.Contains(roster.body, ownerID) || !strings.Contains(roster.body, delegateID) || !strings.Contains(roster.body, readerID) {
		t.Fatalf("share roster exposed the protected owner or hid a delegate: %s", roster.body)
	}
	for _, attempt := range []struct {
		name, action, fields string
	}{
		{"owner email revoke", "share_user", `"recipient_email":"prepare-owner@test.local","permission":0`},
		{"owner email downgrade", "share_user", fmt.Sprintf(`"recipient_email":"prepare-owner@test.local","permission":%d`, auth.GroupRead)},
		{"owner ID revoke", "share_user", fmt.Sprintf(`"user_account_id":%q,"permission":0`, ownerID)},
		{"owner ID downgrade", "share_user", fmt.Sprintf(`"user_account_id":%q,"permission":%d`, ownerID, auth.GroupRead)},
		{"owner group revoke", "share", fmt.Sprintf(`"usergroup_id":%q,"permission":0`, ownerGroup[1])},
		{"owner group downgrade", "share", fmt.Sprintf(`"usergroup_id":%q,"permission":%d`, ownerGroup[1], auth.GroupCRUD)},
	} {
		response := davE2ERequest(client, http.MethodPost, base+"/action/collection/"+attempt.action, ownerToken,
			"application/json", fmt.Sprintf(`{"attributes":{"calendar_reference_id":%q,%s}}`, collectionID, attempt.fields), nil)
		if response.err != nil || response.status != http.StatusConflict {
			t.Fatalf("%s changed the protected owner share: %+v", attempt.name, response)
		}
	}
	if response := call("share_capabilities", ownerToken); response.err != nil || !strings.Contains(response.body, `"revocable_edit_sharing":true`) {
		t.Fatalf("attempted owner share removal changed readiness: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+createdID, ownerToken, "", "", nil), http.StatusOK)
	afterAttemptURL := calendarURL + "after-attempt.ics"
	afterAttemptBody := strings.Replace(davE2ECalendarB, "UID:dav-condition-test", "UID:prepare-after-attempt", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, afterAttemptURL, delegateToken, "text/calendar", afterAttemptBody, nil), http.StatusCreated)
	afterAttemptID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "calendar", "rpath",
		"/caldav/"+owner[1]+"/calendars/prepared/after-attempt.ics")
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, afterAttemptURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+afterAttemptID, ownerToken, "", "", nil), http.StatusOK)
	created := accessGroupsE2ERequestJSON(t, client, http.MethodGet, base+"/api/calendar/"+createdID,
		adminToken, nil, http.StatusOK)
	createdAttributes := created.(map[string]interface{})["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	if createdAttributes["user_account_id"] != delegateID || createdAttributes["permission"] != float64(auth.None) {
		t.Fatalf("migration changed the event owner or retained creator rights: %#v", createdAttributes)
	}
	for _, token := range []string{ownerToken, delegateToken, readerToken} {
		davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, token, "", "", nil), http.StatusOK)
	}
	share(delegateID, 0)
	for _, path := range []string{createdURL, base + "/api/calendar/" + createdID, legacyURL, base + "/api/calendar/" + legacyID} {
		if response := davE2ERequest(client, http.MethodGet, path, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
			t.Fatalf("revoked editor read %s: %+v", path, response)
		}
	}
	for name, token := range map[string]string{"owner": ownerToken, "reader": readerToken} {
		davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, token, "", "", nil), http.StatusOK)
		response := davE2ERequest(client, http.MethodGet, base+"/api/calendar/"+createdID, token, "", "", nil)
		if response.err != nil || response.status != http.StatusOK {
			t.Fatalf("%s lost direct event read after editor revocation: %+v", name, response)
		}
	}
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		body := ""
		if method == http.MethodPatch {
			body = fmt.Sprintf(`{"data":{"type":"calendar","id":%q,"attributes":{"rpath":"/denied.ics"}}}`, createdID)
		}
		if response := davE2ERequest(client, method, base+"/api/calendar/"+createdID, delegateToken,
			"application/vnd.api+json", body, nil); response.err != nil || response.status == http.StatusOK || response.status == http.StatusNoContent {
			t.Fatalf("revoked editor changed event with %s: %+v", method, response)
		}
	}
}
