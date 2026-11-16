package main

import (
	"fmt"
	"github.com/daptin/daptin/server/auth"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestAdvisoryGWCWDeletionDeniedRealE2E(t *testing.T) {
	requireRealE2E(t)
	databaseType, connectionString := "sqlite3", filepath.Join(t.TempDir(), "advisory.db")

	t.Helper()
	usedPorts := make(map[int]bool)
	client := &http.Client{Timeout: 30 * time.Second}
	start := func() (string, *transportE2EDaptinProcess) {
		port := freeTransportE2EPort(t, usedPorts)
		httpsPort := freeTransportE2EPort(t, usedPorts)
		olricPort := freeTransportE2EPortPair(t, usedPorts)
		base := fmt.Sprintf("http://127.0.0.1:%d", port)
		process := startTransportE2EDaptin(t, port, httpsPort, base, transportE2EDaptinOptions{
			databaseType: databaseType, connectionString: connectionString, olricPort: olricPort, schema: davCardACLE2ESchema,
		})
		return base, process
	}
	firstURL, first := start()
	adminToken := transportE2ESignupSigninAdmin(t, client, firstURL)
	ftpE2ESetConfig(t, client, firstURL, adminToken, "caldav.enable", "true")
	first.stopProcess()
	base, process := start()
	defer process.stopProcess()
	ownerToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "card-acl-owner")
	delegateToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "card-acl-delegate")
	outsiderToken := accessGroupsE2ESignupSigninUser(t, client, base, adminToken, "card-acl-outsider")
	principal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/carddav/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-principal/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	owner := regexp.MustCompile(`/carddav/([0-9a-f-]{36})/`).FindStringSubmatch(principal.body)
	if len(owner) != 2 {
		t.Fatalf("missing CardDAV principal: %s", principal.body)
	}
	bookURL := base + "/carddav/" + owner[1] + "/addressbooks/team/"
	contactURL := bookURL + "one.vcf"
	mkcol := `<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:set><D:prop><D:resourcetype><D:collection/><C:addressbook/></D:resourcetype><D:displayname>Team contacts</D:displayname><C:addressbook-description>First description</C:addressbook-description></D:prop></D:set></D:mkcol>`
	davE2EExpect(t, davE2ERequest(client, "MKCOL", bookURL, ownerToken, "application/xml", mkcol, nil), http.StatusCreated)
	props := davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:displayname/><C:addressbook-description/><D:current-user-privilege-set/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(props.body, "Team contacts") || !strings.Contains(props.body, "First description") {
		t.Fatalf("MKCOL properties were not stored: %s", props.body)
	}
	patch := `<D:propertyupdate xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:set><D:prop><D:displayname>Team directory</D:displayname><C:addressbook-description>Updated description</C:addressbook-description></D:prop></D:set></D:propertyupdate>`
	davE2EExpect(t, davE2ERequest(client, "PROPPATCH", bookURL, ownerToken, "application/xml", patch, nil), http.StatusMultiStatus)
	props = davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:displayname/><C:addressbook-description/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(props.body, "Team directory") || !strings.Contains(props.body, "Updated description") {
		t.Fatalf("PROPPATCH properties were not stored: %s", props.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, contactURL, ownerToken, "text/vcard", davE2ECardA, nil), http.StatusCreated)
	if response := davE2ERequest(client, http.MethodGet, contactURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("unshared delegate read contact: %+v", response)
	}
	groupID := accessGroupsE2ECreateUsergroup(t, client, base, adminToken, "card-acl-team")
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/carddav/groups/"+groupID+"/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:principal-URL/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	usersID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "usergroup", "name", "users")
	delegateID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "user_account", "email", "card-acl-delegate@test.local")
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/carddav/"+delegateID+"/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:principal-URL/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	calendarPrincipal := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/caldav/"+delegateID+"/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:principal-URL/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(calendarPrincipal.body, "/caldav/"+delegateID+"/") {
		t.Fatalf("shared calendar account principal was not discoverable: %s", calendarPrincipal.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodPatch, base+"/api/user_account/"+delegateID+"/relationships/usergroup_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q},{"type":"usergroup","id":%q}]}`, usersID, groupID), nil), http.StatusNoContent)
	bookID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "address_book", "name", "team")
	share := func(grant auth.AuthPermission) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/address_book/share", ownerToken, "application/json",
			fmt.Sprintf(`{"attributes":{"address_book_reference_id":%q,"usergroup_id":%q,"permission":%d}}`, bookID, groupID, grant), nil)
	}
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek), http.StatusOK)
	privileges := davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, delegateToken, "application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:current-user-privilege-set/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	t.Logf("read-only privileges: %s", privileges.body)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, contactURL, delegateToken, "text/vcard", davE2ECardA, nil), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, contactURL, delegateToken, "", "", nil), http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL, outsiderToken, "", "", nil), http.StatusNotFound)
	// Update permission must not imply permission to delete a contact.
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek|auth.GroupUpdate), http.StatusOK)
	result := davE2ERequest(client, http.MethodDelete, contactURL, delegateToken, "", "", nil)
	t.Logf("write-content without delete: DELETE contact status=%d err=%v", result.status, result.err)
	davE2EExpect(t, result, http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek), http.StatusOK)
	for i := 2; i <= 3; i++ {
		card := strings.Replace(davE2ECardA, "UID:dav-condition-test", fmt.Sprintf("UID:advisory-%d", i), 1)
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, fmt.Sprintf("%sc%d.vcf", bookURL, i), ownerToken, "text/vcard", card, nil), http.StatusCreated)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+"/api/address_book/"+bookID, delegateToken, "", "", nil), http.StatusForbidden)
	result = davE2ERequest(client, http.MethodDelete, bookURL, delegateToken, "", "", nil)
	t.Logf("read-only delegate: DELETE collection status=%d err=%v", result.status, result.err)
	davE2EExpect(t, result, http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken, "application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	for _, target := range []string{contactURL, bookURL + "c2.vcf", bookURL + "c3.vcf"} {
		davE2EExpect(t, davE2ERequest(client, http.MethodGet, target, ownerToken, "", "", nil), http.StatusOK)
	}
	t.Log("Confirmed read-only and update-only deletion is denied; the collection and all three contacts remain readable by their owner")
}
