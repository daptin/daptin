package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"testing"
)

func TestDAVACLMissingAccountReferRealE2E(t *testing.T) {
	requireRealE2E(t)
	usedPorts := make(map[int]bool)
	databasePath := filepath.Join(t.TempDir(), "dav-acl-missing-refer.db")
	client := &http.Client{}
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
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-acl-owner")
	delegateToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "dav-acl-delegate")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/caldav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing owner principal: %s", principal.body)
	}
	delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "dav-acl-delegate@test.local")
	collectionURL := base + "/caldav/" + owner[1] + "/calendars/private/"
	eventURL := collectionURL + "event.ics"
	davE2EExpect(t, davE2ERequest(client, "MKCOL", collectionURL, ownerToken, "", "", nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, eventURL, ownerToken,
		"text/calendar", davE2ECalendarA, nil), http.StatusCreated)
	acl := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>%s/caldav/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace></D:acl>`, base, delegateID)
	response := davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", acl, nil)
	if response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("ACL without account reference permission returned %+v", response)
	}
	if response := davE2ERequest(client, http.MethodGet, eventURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("denied ACL exposed event: %+v", response)
	}
	groupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "dav-acl-unreferable")
	accessGroupsE2EAssertStatus(t, client, http.MethodPatch, base+"/api/usergroup/"+groupID, adminToken,
		accessGroupsE2ERecordPayload("usergroup", groupID, map[string]interface{}{"permission": 0}), http.StatusOK)
	groupACL := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/caldav/groups/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace></D:acl>`, groupID)
	response = davE2ERequest(client, "ACL", collectionURL, ownerToken, "application/xml", groupACL, nil)
	if response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("ACL without group reference permission returned %+v", response)
	}
}
