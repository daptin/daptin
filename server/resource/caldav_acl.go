package resource

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/caldav"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

var _ caldav.CalendarACLBackend = (*DaptinDAVBackend)(nil)

const calDAVPrivilegeNamespace = "urn:ietf:params:xml:ns:caldav"

func calendarACLLinkGrant(value interface{}) (auth.AuthPermission, error) {
	bits, err := ResourceRowInt64(value)
	if err != nil || bits < 0 {
		return 0, errors.New("invalid calendar group permission")
	}
	const groupRights = auth.GroupPeek | auth.GroupRead | auth.GroupCreate | auth.GroupUpdate | auth.GroupDelete | auth.GroupExecute | auth.GroupRefer
	return auth.AuthPermission(bits) & groupRights, nil
}

func calendarACLPrivileges(grant auth.AuthPermission) []xml.Name {
	var names []xml.Name
	add := func(bit auth.AuthPermission, space, local string) {
		if grant&bit == bit {
			names = append(names, xml.Name{Space: space, Local: local})
		}
	}
	if grant&auth.GroupRead != 0 {
		add(auth.GroupRead, "DAV:", "read")
	} else {
		add(auth.GroupPeek, calDAVPrivilegeNamespace, "read-free-busy")
	}
	add(auth.GroupUpdate, "DAV:", "write-content")
	add(auth.GroupCreate|auth.GroupRefer, "DAV:", "bind")
	add(auth.GroupDelete|auth.GroupUpdate, "DAV:", "unbind")
	add(auth.GroupExecute, "DAV:", "write-acl")
	return names
}

func calendarACLGrant(names []xml.Name) (auth.AuthPermission, error) {
	var grant auth.AuthPermission
	seen := make(map[xml.Name]bool)
	for _, name := range names {
		if seen[name] {
			return 0, errors.New("duplicate DAV privilege")
		}
		seen[name] = true
		switch name {
		case xml.Name{Space: calDAVPrivilegeNamespace, Local: "read-free-busy"}:
			grant |= auth.GroupPeek
		case xml.Name{Space: "DAV:", Local: "read"}:
			grant |= auth.GroupRead | auth.GroupPeek
		case xml.Name{Space: "DAV:", Local: "write-content"}:
			grant |= auth.GroupUpdate
		case xml.Name{Space: "DAV:", Local: "bind"}:
			grant |= auth.GroupCreate | auth.GroupRefer
		case xml.Name{Space: "DAV:", Local: "unbind"}:
			grant |= auth.GroupDelete
		case xml.Name{Space: "DAV:", Local: "write-acl"}:
			grant |= auth.GroupExecute
		default:
			return 0, fmt.Errorf("unsupported DAV privilege %s", name.Local)
		}
	}
	if grant&(auth.GroupUpdate|auth.GroupCreate|auth.GroupDelete|auth.GroupExecute) != 0 && grant&auth.GroupRead == 0 {
		return 0, errors.New("calendar write privileges require read")
	}
	if grant&auth.GroupDelete != 0 && grant&auth.GroupUpdate == 0 {
		return 0, errors.New("DAV:unbind requires DAV:write-content")
	}
	return grant, nil
}

func calendarACLRepresentable(grant auth.AuthPermission) bool {
	actual, err := calendarACLGrant(calendarACLPrivileges(grant))
	return err == nil && actual == grant
}

func equalCalendarACLPrivileges(a, b []xml.Name) bool {
	if len(a) != len(b) {
		return false
	}
	set := make(map[xml.Name]bool, len(a))
	for _, name := range a {
		set[name] = true
	}
	if len(set) != len(a) {
		return false
	}
	for _, name := range b {
		if !set[name] {
			return false
		}
	}
	return true
}

func calendarACLPrincipal(href string) (string, daptinid.DaptinReferenceId, error) {
	u, err := url.Parse(href)
	if err != nil || u.Host != "" || u.Scheme != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", daptinid.NullReferenceId, errors.New("invalid local DAV principal")
	}
	parts := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
	if len(parts) == 2 && parts[0] == "caldav" {
		ref := daptinid.InterfaceToDIR(parts[1])
		if ref != daptinid.NullReferenceId {
			return "account", ref, nil
		}
	}
	if len(parts) == 3 && parts[0] == "caldav" && parts[1] == "groups" {
		ref := daptinid.InterfaceToDIR(parts[2])
		if ref != daptinid.NullReferenceId {
			return "group", ref, nil
		}
	}
	return "", daptinid.NullReferenceId, errors.New("unknown local DAV principal")
}

func (b *DaptinDAVBackend) GetGroupPrincipal(_ context.Context, requestPath string) error {
	kind, ref, err := calendarACLPrincipal(requestPath)
	if err != nil || kind != "group" {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("group principal not found"))
	}
	tx, err := b.cruds["usergroup"].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := GetReferenceIdToIdWithTransaction("usergroup", ref, tx); err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	admin := b.cruds["usergroup"].AdministratorGroupId
	table := b.cruds["usergroup"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "usergroup", tx)
	row := GetObjectPermissionByReferenceIdWithTransaction("usergroup", ref, tx)
	if !table.CanRefer(b.sessionUser.UserReferenceId, b.sessionUser.Groups, admin) ||
		!row.CanRefer(b.sessionUser.UserReferenceId, b.sessionUser.Groups, admin) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("group principal denied"))
	}
	return tx.Commit()
}

func (b *DaptinDAVBackend) calendarACLCollection(requestPath string, tx *sqlx.Tx) (map[string]interface{}, daptinid.DaptinReferenceId, daptinid.DaptinReferenceId, error) {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return nil, daptinid.NullReferenceId, daptinid.NullReferenceId, err
	}
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return nil, daptinid.NullReferenceId, daptinid.NullReferenceId, err
	}
	return collection, daptinid.InterfaceToDIR(collection["reference_id"]), owner, nil
}

func (b *DaptinDAVBackend) calendarACLCanWrite(ref daptinid.DaptinReferenceId, tx *sqlx.Tx) (bool, error) {
	share := &calendarShareAction{cruds: b.cruds}
	active, canManage := share.managementAccess(ref, b.sessionUser, tx)
	if !canManage {
		return false, nil
	}
	return b.calendarACLActionAllowed("share", active, tx)
}

func (b *DaptinDAVBackend) calendarACLActionAllowed(name string, active *auth.SessionUser, tx *sqlx.Tx) (bool, error) {
	action, err := b.cruds[calendarCollectionTable].GetActionByName(calendarCollectionTable, name, tx)
	if err != nil {
		return false, err
	}
	grant := GetObjectPermissionByReferenceIdWithTransaction("action", action.ReferenceId, tx)
	return IsAdminWithTransaction(active, tx) || grant.CanExecute(active.UserReferenceId, active.Groups, b.cruds[calendarCollectionTable].AdministratorGroupId), nil
}

func (b *DaptinDAVBackend) calendarACLProtectedGroups(owner daptinid.DaptinReferenceId, tx *sqlx.Tx) (map[int64]bool, error) {
	ownerID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, owner, tx)
	if err != nil {
		return nil, err
	}
	members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx,
		goqu.Ex{"user_account_id": ownerID})
	if err != nil {
		return nil, err
	}
	protected := make(map[int64]bool, len(members)+1)
	for _, member := range members {
		groupID, ok := member["usergroup_id"].(int64)
		if !ok {
			return nil, errors.New("invalid owner group membership")
		}
		protected[groupID] = true
	}
	adminID, err := GetReferenceIdToIdWithTransaction("usergroup", b.cruds[calendarCollectionTable].AdministratorGroupId, tx)
	if err != nil {
		return nil, err
	}
	protected[adminID] = true
	return protected, nil
}

func (b *DaptinDAVBackend) calendarACLLinks(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return nil, err
	}
	return GetObjectByWhereClauseWithTransaction("collection_collection_id_has_usergroup_usergroup_id", tx, goqu.Ex{"collection_id": collectionID})
}

func (b *DaptinDAVBackend) aclLinkPrincipal(collectionRef, groupRef daptinid.DaptinReferenceId, groupID int64, tx *sqlx.Tx) (string, error) {
	group, _, err := b.cruds["usergroup"].GetSingleRowByReferenceIdWithTransaction("usergroup", groupRef, nil, tx)
	if err != nil {
		return "", err
	}
	prefix := "calshare:" + strings.ReplaceAll(collectionRef.String(), "-", "") + ":"
	if strings.HasPrefix(StringOrEmpty(group["name"]), prefix) {
		members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
		if err != nil {
			return "", err
		}
		if len(members) == 1 {
			if userID, ok := members[0]["user_account_id"].(int64); ok {
				userRef, err := GetIdToReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, userID, tx)
				if err != nil {
					return "", err
				}
				if strings.ReplaceAll(userRef.String(), "-", "") == strings.TrimPrefix(StringOrEmpty(group["name"]), prefix) {
					return "/caldav/" + userRef.String() + "/", nil
				}
			}
		}
	}
	return "/caldav/groups/" + groupRef.String() + "/", nil
}

func (b *DaptinDAVBackend) GetCalendarACL(_ context.Context, requestPath string) (*caldav.CalendarACL, error) {
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, ref, owner, err := b.calendarACLCollection(requestPath, tx)
	if err != nil {
		return nil, err
	}
	acl := &caldav.CalendarACL{Owner: "/caldav/" + owner.String() + "/"}
	_, canManage := (&calendarShareAction{cruds: b.cruds}).managementAccess(ref, b.sessionUser, tx)
	if !canManage {
		return acl, tx.Commit()
	}
	acl.CanWrite, err = b.calendarACLCanWrite(ref, tx)
	if err != nil {
		return nil, err
	}
	links, err := b.calendarACLLinks(ref, tx)
	if err != nil {
		return nil, err
	}
	protectedGroups, err := b.calendarACLProtectedGroups(owner, tx)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		groupID, ok := link["usergroup_id"].(int64)
		if !ok {
			return nil, errors.New("invalid calendar group link")
		}
		groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
		if err != nil {
			return nil, err
		}
		grant, err := calendarACLLinkGrant(link["permission"])
		if err != nil {
			return nil, err
		}
		privileges := calendarACLPrivileges(grant)
		if len(privileges) == 0 {
			continue
		}
		principal, err := b.aclLinkPrincipal(ref, groupRef, groupID, tx)
		if err != nil {
			return nil, err
		}
		acl.Entries = append(acl.Entries, caldav.CalendarACE{Principal: principal, Privileges: privileges, Protected: protectedGroups[groupID] || !calendarACLRepresentable(grant)})
	}
	return acl, tx.Commit()
}

func (b *DaptinDAVBackend) ReplaceCalendarACL(_ context.Context, requestPath string, entries []caldav.CalendarACE) error {
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, ref, owner, err := b.calendarACLCollection(requestPath, tx)
	if err != nil {
		return err
	}
	share := &calendarShareAction{cruds: b.cruds}
	collectionID, active, err := share.authorizeCollection(ref, b.sessionUser, tx)
	if err != nil {
		return davResourceError(err)
	}
	canWrite, err := b.calendarACLCanWrite(ref, tx)
	if err != nil {
		return err
	}
	if !canWrite {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar ACL denied"))
	}
	old, err := b.calendarACLLinks(ref, tx)
	if err != nil {
		return err
	}
	protectedGroups, err := b.calendarACLProtectedGroups(owner, tx)
	if err != nil {
		return err
	}
	protected := make(map[daptinid.DaptinReferenceId]auth.AuthPermission)
	protectedPrincipal := make(map[string]auth.AuthPermission)
	for _, link := range old {
		groupID, ok := link["usergroup_id"].(int64)
		if !ok {
			return errors.New("invalid calendar group link")
		}
		grant, err := calendarACLLinkGrant(link["permission"])
		if err != nil {
			return err
		}
		isProtected := protectedGroups[groupID] || !calendarACLRepresentable(grant)
		if isProtected {
			groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
			if err != nil {
				return err
			}
			protected[groupRef] = grant
			principal, err := b.aclLinkPrincipal(ref, groupRef, groupID, tx)
			if err != nil {
				return err
			}
			protectedPrincipal[principal] = grant
		}
	}
	target := make(map[daptinid.DaptinReferenceId]bool)
	for _, entry := range entries {
		kind, principalRef, err := calendarACLPrincipal(entry.Principal)
		if err != nil {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("invalid calendar ACL principal"))
		}
		if oldGrant, ok := protectedPrincipal[entry.Principal]; ok {
			if !entry.Protected || !equalCalendarACLPrivileges(entry.Privileges, calendarACLPrivileges(oldGrant)) {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("protected ACL changed"))
			}
			continue
		}
		grant, err := calendarACLGrant(entry.Privileges)
		if err != nil || grant == 0 {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("unsupported calendar ACL grant"))
		}
		if principalRef == owner {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar owner ACL is protected"))
		}
		var groupRef daptinid.DaptinReferenceId
		if kind == "account" {
			allowed, err := b.calendarACLActionAllowed("share_user", active, tx)
			if err != nil {
				return err
			}
			if !allowed {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("account sharing action denied"))
			}
			if entry.Protected {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("unexpected protected ACE"))
			}
			groupRef, err = share.shareWithUser(ref, principalRef, grant, b.sessionUser, tx)
			if err != nil {
				return davResourceError(err)
			}
		} else {
			groupRef = principalRef
			if entry.Protected {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("unexpected protected ACE"))
			}
			groupID, err := GetReferenceIdToIdWithTransaction("usergroup", groupRef, tx)
			if err != nil {
				return webdav.NewHTTPError(http.StatusNotFound, err)
			}
			if protectedGroups[groupID] {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("owner or administrator ACL is protected"))
			}
			err = share.applyAuthorized(collectionID, ref, groupRef, grant, b.sessionUser, active, tx)
			if err != nil {
				return davResourceError(err)
			}
		}
		if target[groupRef] {
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("duplicate calendar ACL principal"))
		}
		target[groupRef] = true
	}
	for _, link := range old {
		groupID, ok := link["usergroup_id"].(int64)
		if !ok {
			return errors.New("invalid calendar group link")
		}
		groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
		if err != nil {
			return err
		}
		if target[groupRef] {
			continue
		}
		if _, keep := protected[groupRef]; keep {
			continue
		}
		if err := share.applyAuthorized(collectionID, ref, groupRef, 0, b.sessionUser, active, tx); err != nil {
			return davResourceError(err)
		}
	}
	return tx.Commit()
}
