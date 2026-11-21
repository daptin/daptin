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

var davCardACLE2ESchema = davCardACLTestSchema(auth.GroupCRUD|auth.GroupExecute, auth.GroupCRUD)

func davCardACLTestSchema(bookTableGrant, contactTableGrant auth.AuthPermission) string {
	return fmt.Sprintf(`Tables:
  - TableName: address_book
    DefaultPermission: %d
    AccessGroups:
      - Name: users
        Permission: %d
  - TableName: contact
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
`, auth.UserCRUD|auth.UserExecute, bookTableGrant,
		auth.UserCRUD, contactTableGrant, auth.GuestRefer, auth.GuestRefer,
		auth.GroupRefer, auth.GroupRefer)
}

func TestDAVCardACLRealE2E(t *testing.T) {
	requireRealE2E(t)
	runDAVCardACLRealE2E(t, "sqlite3", filepath.Join(t.TempDir(), "dav-card-acl.db"))
}

func TestDAVCardACLPostgresRealE2E(t *testing.T) {
	requireRealE2E(t)
	dsn := os.Getenv("DAPTIN_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set DAPTIN_TEST_POSTGRES_DSN to a disposable PostgreSQL database")
	}
	runDAVCardACLRealE2E(t, "postgres", dsn)
}

func runDAVCardACLRealE2E(t *testing.T, databaseType, connectionString string) {
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
	// Collection deletion must not turn read or update access into delete access.
	contactURLs := []string{contactURL, bookURL + "two.vcf", bookURL + "three.vcf"}
	for i, target := range contactURLs[1:] {
		card := strings.Replace(davE2ECardA, "UID:dav-condition-test", fmt.Sprintf("UID:card-delete-%d", i), 1)
		davE2EExpect(t, davE2ERequest(client, http.MethodPut, target, ownerToken, "text/vcard", card, nil), http.StatusCreated)
	}
	assertIntact := func() {
		t.Helper()
		davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken, "application/xml",
			`<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
		for _, target := range contactURLs {
			davE2EExpect(t, davE2ERequest(client, http.MethodGet, target, ownerToken, "", "", nil), http.StatusOK)
		}
	}
	assertDeleteDenied := func(token string, status int) {
		t.Helper()
		before := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, ownerToken, "application/xml",
			`<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, nil), http.StatusMultiStatus)
		checkpoint := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(before.body)
		if len(checkpoint) != 2 {
			t.Fatalf("missing sync checkpoint before denied deletion: %s", before.body)
		}
		davE2EExpect(t, davE2ERequest(client, http.MethodDelete, contactURL, token, "", "", nil), status)
		davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL, token, "", "", nil), status)
		assertIntact()
		after := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, ownerToken, "application/xml",
			fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, checkpoint[1]), nil), http.StatusMultiStatus)
		if strings.Contains(after.body, ".vcf") {
			t.Fatalf("denied deletion changed contact sync history: %s", after.body)
		}
	}
	assertDeleteDenied(outsiderToken, http.StatusNotFound)
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek), http.StatusOK)
	assertDeleteDenied(delegateToken, http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+"/api/address_book/"+bookID, delegateToken, "", "", nil), http.StatusForbidden)
	delegateACL := davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, delegateToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:acl/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(delegateACL.body, "/carddav/groups/"+groupID+"/") {
		t.Fatalf("readable address-book ACL omitted the active share: %s", delegateACL.body)
	}
	groupProperties := davE2EExpect(t, davE2ERequest(client, "PROPFIND", base+"/carddav/groups/"+groupID+"/", ownerToken,
		"application/xml", `<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/><D:group-member-set/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(groupProperties.body, "card-acl-team") || !strings.Contains(groupProperties.body, "/carddav/"+delegateID+"/") {
		t.Fatalf("CardDAV group principal properties are incomplete: %s", groupProperties.body)
	}
	searchProperties := davE2EExpect(t, davE2ERequest(client, "REPORT", base+"/carddav/", ownerToken,
		"application/xml", `<D:principal-search-property-set xmlns:D="DAV:"/>`, nil), http.StatusOK)
	if !strings.Contains(searchProperties.body, "displayname") {
		t.Fatalf("CardDAV principal search properties are missing: %s", searchProperties.body)
	}
	search := davE2EExpect(t, davE2ERequest(client, "REPORT", base+"/carddav/", ownerToken,
		"application/xml", `<D:principal-property-search xmlns:D="DAV:"><D:property-search><D:prop><D:displayname/></D:prop><D:match>card-acl-delegate</D:match></D:property-search><D:prop><D:displayname/></D:prop></D:principal-property-search>`, nil), http.StatusMultiStatus)
	if !strings.Contains(search.body, "/carddav/"+delegateID+"/") {
		t.Fatalf("CardDAV principal search omitted delegate: %s", search.body)
	}
	matchPrincipal := davE2EExpect(t, davE2ERequest(client, "REPORT", base+"/carddav/", delegateToken,
		"application/xml", `<D:principal-match xmlns:D="DAV:"><D:self/><D:prop><D:displayname/></D:prop></D:principal-match>`, nil), http.StatusMultiStatus)
	if !strings.Contains(matchPrincipal.body, "/carddav/"+delegateID+"/") {
		t.Fatalf("CardDAV principal-match omitted the active account: %s", matchPrincipal.body)
	}
	aclPrincipals := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, ownerToken,
		"application/xml", `<D:acl-principal-prop-set xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:acl-principal-prop-set>`, nil), http.StatusMultiStatus)
	if !strings.Contains(aclPrincipals.body, "card-acl-team") {
		t.Fatalf("CardDAV ACL principal report omitted shared group: %s", aclPrincipals.body)
	}
	expanded := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, ownerToken,
		"application/xml", `<D:expand-property xmlns:D="DAV:"><D:property name="owner"><D:property name="displayname"/></D:property></D:expand-property>`, nil), http.StatusMultiStatus)
	if strings.Count(expanded.body, "<response") < 2 || !strings.Contains(expanded.body, "card-acl-owner") {
		t.Fatalf("CardDAV expanded owner lacked principal properties: %s", expanded.body)
	}
	deniedACE := davE2EExpect(t, davE2ERequest(client, "ACL", bookURL, ownerToken,
		"application/xml", fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/carddav/groups/%s/</D:href></D:principal><D:deny><D:privilege><D:read/></D:privilege></D:deny></D:ace></D:acl>`, groupID), nil), http.StatusForbidden)
	if !strings.Contains(deniedACE.body, "grant-only") {
		t.Fatalf("CardDAV ACL denial omitted its precondition: %s", deniedACE.body)
	}
	abstractACE := davE2EExpect(t, davE2ERequest(client, "ACL", bookURL, ownerToken,
		"application/xml", fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/carddav/groups/%s/</D:href></D:principal><D:grant><D:privilege><D:all/></D:privilege></D:grant></D:ace></D:acl>`, groupID), nil), http.StatusForbidden)
	if !strings.Contains(abstractACE.body, "no-abstract") {
		t.Fatalf("CardDAV abstract ACL privilege omitted its precondition: %s", abstractACE.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, delegateToken, "", "", nil), http.StatusOK)
	query := `<C:addressbook-query xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:getetag/><C:address-data/></D:prop><C:filter><C:prop-filter name="FN"/></C:filter></C:addressbook-query>`
	queryResult := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, delegateToken, "application/xml", query, nil), http.StatusMultiStatus)
	if !strings.Contains(queryResult.body, "one.vcf") || !strings.Contains(queryResult.body, "BEGIN:VCARD") {
		t.Fatalf("shared address-book query omitted contact: %s", queryResult.body)
	}
	multiget := fmt.Sprintf(`<C:addressbook-multiget xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:prop><D:getetag/><C:address-data/></D:prop><D:href>/carddav/%s/addressbooks/team/one.vcf</D:href></C:addressbook-multiget>`, owner[1])
	multigetResult := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, delegateToken, "application/xml", multiget, nil), http.StatusMultiStatus)
	if !strings.Contains(multigetResult.body, "one.vcf") || !strings.Contains(multigetResult.body, "BEGIN:VCARD") {
		t.Fatalf("shared address-book multiget omitted contact: %s", multigetResult.body)
	}
	syncResult := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, delegateToken, "application/xml", `<D:sync-collection xmlns:D="DAV:"><D:sync-token/><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, nil), http.StatusMultiStatus)
	if !strings.Contains(syncResult.body, "one.vcf") || !strings.Contains(syncResult.body, "sync-token") {
		t.Fatalf("shared address-book sync omitted contact: %s", syncResult.body)
	}
	syncToken := regexp.MustCompile(`<sync-token[^>]*>([^<]+)</sync-token>`).FindStringSubmatch(syncResult.body)
	if len(syncToken) != 2 {
		t.Fatalf("shared address-book sync returned no usable token: %s", syncResult.body)
	}
	contactID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "contact", "rpath", "/carddav/"+owner[1]+"/addressbooks/team/one.vcf")
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, base+"/api/contact/"+contactID, delegateToken, "", "", nil), http.StatusOK)
	// Revoke only this contact's shared group. Book access and sibling contacts
	// remain readable; neither a known href nor multiget may bypass revocation.
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, base+"/api/contact/"+contactID+"/relationships/usergroup_id", adminToken,
		"application/vnd.api+json", fmt.Sprintf(`{"data":[{"type":"usergroup","id":%q}]}`, groupID), nil), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, delegateToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, delegateToken, "", "", nil), http.StatusNotFound)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, bookURL+"two.vcf", delegateToken, "", "", nil), http.StatusOK)
	queryResult = davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, delegateToken, "application/xml", query, nil), http.StatusMultiStatus)
	if strings.Contains(queryResult.body, "one.vcf") || !strings.Contains(queryResult.body, "two.vcf") {
		t.Fatalf("contact query did not enforce individual revocation: %s", queryResult.body)
	}
	mixedMultiget := strings.Replace(multiget, "</C:addressbook-multiget>",
		fmt.Sprintf(`<D:href>/carddav/%s/addressbooks/team/two.vcf</D:href></C:addressbook-multiget>`, owner[1]), 1)
	multigetResult = davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, delegateToken, "application/xml", mixedMultiget, nil), http.StatusMultiStatus)
	if strings.Contains(multigetResult.body, "UID:dav-condition-test") || !strings.Contains(multigetResult.body, "UID:card-delete-0") || !strings.Contains(multigetResult.body, "404") {
		t.Fatalf("multiget leaked revoked contact or omitted readable sibling: %s", multigetResult.body)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, ownerToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, adminToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, delegateToken, "", "", nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, contactURL, outsiderToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("share exposed contact to outsider: %+v", response)
	}
	if response := davE2ERequest(client, "PROPPATCH", bookURL, delegateToken, "application/xml", patch, nil); response.err != nil || !strings.Contains(response.body, "403") {
		t.Fatalf("read-only delegate changed book properties: %+v", response)
	}
	if response := davE2ERequest(client, "ACL", bookURL, delegateToken, "application/xml", `<D:acl xmlns:D="DAV:"/>`, nil); response.err != nil || response.status != http.StatusForbidden {
		t.Fatalf("read-only delegate replaced address-book ACL: %+v", response)
	}
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek|auth.GroupUpdate), http.StatusOK)
	assertDeleteDenied(delegateToken, http.StatusForbidden)
	staleSync := fmt.Sprintf(`<D:sync-collection xmlns:D="DAV:"><D:sync-token>%s</D:sync-token><D:sync-level>1</D:sync-level><D:prop><D:getetag/></D:prop></D:sync-collection>`, syncToken[1])
	invalid := davE2EExpect(t, davE2ERequest(client, "REPORT", bookURL, delegateToken, "application/xml", staleSync, nil), http.StatusForbidden)
	if !strings.Contains(invalid.body, "valid-sync-token") {
		t.Fatalf("changed address-book access retained an old sync token: %s", invalid.body)
	}
	davE2EExpect(t, share(0), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, base+"/api/contact/"+contactID, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("revoked delegate retained JSON:API contact access: %+v", response)
	}
	shareUser := func(grant auth.AuthPermission) davE2EResponse {
		return davE2ERequest(client, http.MethodPost, base+"/action/address_book/share_user", ownerToken, "application/json",
			fmt.Sprintf(`{"attributes":{"address_book_reference_id":%q,"user_account_id":%q,"permission":%d}}`, bookID, delegateID, grant), nil)
	}
	davE2EExpect(t, shareUser(auth.GroupRead|auth.GroupPeek), http.StatusOK)
	assertDeleteDenied(delegateToken, http.StatusForbidden)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, contactURL, delegateToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, shareUser(0), http.StatusOK)
	assertDeleteDenied(delegateToken, http.StatusNotFound)
	readACL := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/carddav/groups/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege></D:grant></D:ace></D:acl>`, groupID)
	davE2EExpect(t, davE2ERequest(client, "ACL", bookURL, ownerToken, "application/xml", readACL, nil), http.StatusOK)
	assertDeleteDenied(delegateToken, http.StatusForbidden)
	acl := fmt.Sprintf(`<D:acl xmlns:D="DAV:"><D:ace><D:principal><D:href>/carddav/groups/%s/</D:href></D:principal><D:grant><D:privilege><D:read/></D:privilege><D:privilege><D:write-content/></D:privilege><D:privilege><D:write-properties/></D:privilege><D:privilege><D:bind/></D:privilege><D:privilege><D:unbind/></D:privilege></D:grant></D:ace></D:acl>`, groupID)
	davE2EExpect(t, davE2ERequest(client, "ACL", bookURL, ownerToken, "application/xml", acl, nil), http.StatusOK)
	createdURL := bookURL + "delegate.vcf"
	createdCard := strings.Replace(davE2ECardA, "UID:dav-condition-test", "UID:card-acl-delegate", 1)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, createdURL, delegateToken, "text/vcard", createdCard, nil), http.StatusCreated)
	// The book owner has no row grant on this delegate-owned contact.
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, ownerToken, "", "", nil), http.StatusNotFound)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, adminToken, "", "", nil), http.StatusOK)
	// An explicit delete grant permits a contact delete through the same backend.
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, createdURL, delegateToken, "", "", nil), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, createdURL, delegateToken, "text/vcard", createdCard, nil), http.StatusCreated)
	davE2EExpect(t, davE2ERequest(client, "PROPPATCH", bookURL, delegateToken, "application/xml", patch, nil), http.StatusMultiStatus)
	aclProps := davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:acl/><D:owner/><D:acl-restrictions/></D:prop></D:propfind>`, nil), http.StatusMultiStatus)
	if !strings.Contains(aclProps.body, "/carddav/groups/"+groupID+"/") || !strings.Contains(aclProps.body, "grant-only") {
		t.Fatalf("CardDAV ACL discovery incomplete: %s", aclProps.body)
	}
	davE2EExpect(t, davE2ERequest(client, "ACL", bookURL, ownerToken, "application/xml", `<D:acl xmlns:D="DAV:"/>`, nil), http.StatusOK)
	if response := davE2ERequest(client, http.MethodGet, createdURL, delegateToken, "", "", nil); response.err != nil || response.status == http.StatusOK {
		t.Fatalf("revoked delegate read its contact through CardDAV: %+v", response)
	}
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, createdURL, ownerToken, "", "", nil), http.StatusNotFound)
	// A child without Delete blocks the whole operation, even when hidden from
	// the deleting account. No earlier child or sync history may disappear.
	patchPermission := func(table, ref string, grant auth.AuthPermission) {
		t.Helper()
		accessGroupsE2ERequestJSON(t, client, http.MethodPatch, base+"/api/"+table+"/"+ref, adminToken,
			accessGroupsE2ERecordPayload(table, ref, map[string]interface{}{"permission": int64(grant)}), http.StatusOK)
	}
	lastContactID := accessGroupsE2EFindResourceID(t, client, base, adminToken, "contact", "rpath", "/carddav/"+owner[1]+"/addressbooks/team/three.vcf")
	patchPermission("contact", lastContactID, 0)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, bookURL+"three.vcf", ownerToken, "", "", nil), http.StatusNotFound)
	davE2EExpect(t, davE2ERequest(client, http.MethodGet, bookURL+"three.vcf", adminToken, "", "", nil), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL, ownerToken, "", "", nil), http.StatusForbidden)
	patchPermission("contact", lastContactID, auth.UserCRUD)
	assertIntact()
	patchPermission("contact", contactID, auth.UserRead|auth.UserPeek|auth.UserUpdate)
	assertDeleteDenied(ownerToken, http.StatusForbidden)
	patchPermission("contact", contactID, auth.UserCRUD)
	patchPermission("address_book", bookID, (auth.UserCRUD|auth.UserExecute)&^auth.UserDelete)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL, ownerToken, "", "", nil), http.StatusForbidden)
	assertIntact()
	patchPermission("address_book", bookID, auth.UserCRUD|auth.UserExecute)
	// Revocation of Delete takes effect immediately after an allowed delete.
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek|auth.GroupUpdate|auth.GroupDelete), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, contactURL, delegateToken, "", "", nil), http.StatusNoContent)
	davE2EExpect(t, davE2ERequest(client, http.MethodPut, contactURL, ownerToken, "text/vcard", davE2ECardA, nil), http.StatusCreated)
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek|auth.GroupUpdate), http.StatusOK)
	assertDeleteDenied(delegateToken, http.StatusForbidden)
	davE2EExpect(t, share(auth.GroupRead|auth.GroupPeek|auth.GroupUpdate|auth.GroupDelete), http.StatusOK)
	davE2EExpect(t, davE2ERequest(client, http.MethodDelete, bookURL, delegateToken, "", "", nil), http.StatusNoContent)
	for _, target := range append(contactURLs, createdURL) {
		davE2EExpect(t, davE2ERequest(client, http.MethodGet, target, ownerToken, "", "", nil), http.StatusNotFound)
	}
	davE2EExpect(t, davE2ERequest(client, "PROPFIND", bookURL, ownerToken, "application/xml",
		`<D:propfind xmlns:D="DAV:"><D:prop><D:displayname/></D:prop></D:propfind>`, nil), http.StatusNotFound)
}

func TestDAVCardDeleteTablePermissionRealE2E(t *testing.T) {
	requireRealE2E(t)
	for _, deniedTable := range []string{"address_book", "contact"} {
		t.Run(deniedTable, func(t *testing.T) {
			bookGrant, contactGrant := auth.GroupCRUD|auth.GroupExecute, auth.GroupCRUD
			if deniedTable == "address_book" {
				bookGrant &^= auth.GroupDelete
			} else {
				contactGrant &^= auth.GroupDelete
			}
			options := transportE2EDaptinOptions{
				connectionString: filepath.Join(t.TempDir(), "table-delete.db"),
				schema:           davCardACLTestSchema(bookGrant, contactGrant),
			}
			f := startDaptinE2E(t, options)
			admin := f.SignupAdmin(t)
			ftpE2ESetConfig(t, f.Client, f.URL, admin, "caldav.enable", "true")
			f.Process.stopProcess()
			f = startDaptinE2E(t, options)
			ownerToken := accessGroupsE2ESignupSigninUser(t, f.Client, f.URL, admin, "delete-table-owner")
			ownerID := accessGroupsE2EFindResourceID(t, f.Client, f.URL, admin, "user_account", "email", "delete-table-owner@test.local")
			bookURL := f.URL + "/carddav/" + ownerID + "/addressbooks/team/"
			contactURL := bookURL + "one.vcf"
			mkcol := `<D:mkcol xmlns:D="DAV:" xmlns:C="urn:ietf:params:xml:ns:carddav"><D:set><D:prop><D:resourcetype><D:collection/><C:addressbook/></D:resourcetype></D:prop></D:set></D:mkcol>`
			davE2EExpect(t, davE2ERequest(f.Client, "MKCOL", bookURL, ownerToken, "application/xml", mkcol, nil), http.StatusCreated)
			davE2EExpect(t, davE2ERequest(f.Client, http.MethodPut, contactURL, ownerToken, "text/vcard", davE2ECardA, nil), http.StatusCreated)
			davE2EExpect(t, davE2ERequest(f.Client, http.MethodDelete, bookURL, ownerToken, "", "", nil), http.StatusForbidden)
			if deniedTable == "contact" {
				davE2EExpect(t, davE2ERequest(f.Client, http.MethodDelete, contactURL, ownerToken, "", "", nil), http.StatusForbidden)
			}
			davE2EExpect(t, davE2ERequest(f.Client, http.MethodGet, contactURL, ownerToken, "", "", nil), http.StatusOK)
			// Administrator authority still permits removal of the same rows.
			davE2EExpect(t, davE2ERequest(f.Client, http.MethodDelete, bookURL, admin, "", "", nil), http.StatusNoContent)
			davE2EExpect(t, davE2ERequest(f.Client, http.MethodGet, contactURL, ownerToken, "", "", nil), http.StatusNotFound)
		})
	}
}
