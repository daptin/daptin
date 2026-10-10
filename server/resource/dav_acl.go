package resource

import (
	"context"
	"encoding/xml"
	"errors"
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

type davACLAdapter struct {
	b               *DaptinDAVBackend
	prefix          string
	collectionTable string
	objectTable     string
	groupPrefix     string
}

func davACLLinkGrant(value interface{}) (auth.AuthPermission, error) {
	bits, err := ResourceRowInt64(value)
	if err != nil || bits < 0 {
		return 0, errors.New("invalid DAV group permission")
	}
	const groupRights = auth.GroupPeek | auth.GroupRead | auth.GroupCreate | auth.GroupUpdate | auth.GroupDelete | auth.GroupExecute | auth.GroupRefer
	return auth.AuthPermission(bits) & groupRights, nil
}

func (a *davACLAdapter) share() *davShareAction {
	return &davShareAction{cruds: a.b.cruds, collectionTable: a.collectionTable, objectTable: a.objectTable, groupPrefix: a.groupPrefix}
}

func (a *davACLAdapter) path(requestPath string) (daptinid.DaptinReferenceId, string, error) {
	if a.prefix == "caldav" {
		return a.b.calendarPath(requestPath, false)
	}
	return a.b.addressBookPath(requestPath, false)
}

func (a *davACLAdapter) collectionRow(owner daptinid.DaptinReferenceId, name, requestPath string, tx *sqlx.Tx) (map[string]interface{}, error) {
	if a.prefix == "caldav" {
		return a.b.calendarCollection(owner, name, requestPath, tx)
	}
	return a.b.addressBookWithTransaction(requestPath, false, tx)
}

func (a *davACLAdapter) privileges(grant auth.AuthPermission) []xml.Name {
	if a.prefix == "caldav" {
		return calendarACLPrivileges(grant)
	}
	return addressBookACLPrivileges(grant)
}

func (a *davACLAdapter) grant(names []xml.Name) (auth.AuthPermission, error) {
	if a.prefix == "caldav" {
		return calendarACLGrant(names)
	}
	return addressBookACLGrant(names)
}

func (a *davACLAdapter) representable(grant auth.AuthPermission) bool {
	actual, err := a.grant(a.privileges(grant))
	return err == nil && actual == grant
}

func (a *davACLAdapter) principal(href string) (string, daptinid.DaptinReferenceId, error) {
	u, err := url.Parse(href)
	if err != nil || u.Host != "" || u.Scheme != "" || u.RawQuery != "" || u.Fragment != "" {
		return "", daptinid.NullReferenceId, errors.New("invalid local DAV principal")
	}
	parts := strings.Split(strings.Trim(path.Clean(u.Path), "/"), "/")
	if len(parts) == 2 && parts[0] == a.prefix {
		ref := daptinid.InterfaceToDIR(parts[1])
		if ref != daptinid.NullReferenceId {
			return "account", ref, nil
		}
	}
	if len(parts) == 3 && parts[0] == a.prefix && parts[1] == "groups" {
		ref := daptinid.InterfaceToDIR(parts[2])
		if ref != daptinid.NullReferenceId {
			return "group", ref, nil
		}
	}
	return "", daptinid.NullReferenceId, errors.New("unknown local DAV principal")
}

func (a *davACLAdapter) getGroupPrincipal(_ context.Context, requestPath string) error {
	return a.getPrincipal(requestPath, "group", "usergroup")
}

func (a *davACLAdapter) getAccountPrincipal(_ context.Context, requestPath string) error {
	return a.getPrincipal(requestPath, "account", USER_ACCOUNT_TABLE_NAME)
}

func (a *davACLAdapter) getPrincipal(requestPath, expectedKind, tableName string) error {
	kind, ref, err := a.principal(requestPath)
	if err != nil || kind != expectedKind {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("DAV principal not found"))
	}
	tx, err := a.b.cruds[tableName].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := GetReferenceIdToIdWithTransaction(tableName, ref, tx); err != nil {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	admin := a.b.cruds[tableName].AdministratorGroupId
	table := a.b.cruds[tableName].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", tableName, tx)
	row := GetObjectPermissionByReferenceIdWithTransaction(tableName, ref, tx)
	if !table.CanRefer(a.b.sessionUser.UserReferenceId, a.b.sessionUser.Groups, admin) ||
		!row.CanRefer(a.b.sessionUser.UserReferenceId, a.b.sessionUser.Groups, admin) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("DAV principal denied"))
	}
	return tx.Commit()
}

func (a *davACLAdapter) collection(requestPath string, tx *sqlx.Tx) (map[string]interface{}, daptinid.DaptinReferenceId, daptinid.DaptinReferenceId, error) {
	owner, name, err := a.path(requestPath)
	if err != nil {
		return nil, daptinid.NullReferenceId, daptinid.NullReferenceId, err
	}
	collection, err := a.collectionRow(owner, name, requestPath, tx)
	if err != nil {
		return nil, daptinid.NullReferenceId, daptinid.NullReferenceId, err
	}
	return collection, daptinid.InterfaceToDIR(collection["reference_id"]), owner, nil
}

func (a *davACLAdapter) canWrite(ref daptinid.DaptinReferenceId, tx *sqlx.Tx) (bool, error) {
	share := a.share()
	active, canManage := share.managementAccess(ref, a.b.sessionUser, tx)
	if !canManage {
		return false, nil
	}
	return a.actionAllowed("share", active, tx)
}

func (a *davACLAdapter) actionAllowed(name string, active *auth.SessionUser, tx *sqlx.Tx) (bool, error) {
	action, err := a.b.cruds[a.collectionTable].GetActionByName(a.collectionTable, name, tx)
	if err != nil {
		return false, err
	}
	grant := GetObjectPermissionByReferenceIdWithTransaction("action", action.ReferenceId, tx)
	return IsAdminWithTransaction(active, tx) || grant.CanExecute(active.UserReferenceId, active.Groups, a.b.cruds[a.collectionTable].AdministratorGroupId), nil
}

func (a *davACLAdapter) protectedGroups(owner daptinid.DaptinReferenceId, tx *sqlx.Tx) (map[int64]bool, error) {
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
	adminID, err := GetReferenceIdToIdWithTransaction("usergroup", a.b.cruds[a.collectionTable].AdministratorGroupId, tx)
	if err != nil {
		return nil, err
	}
	protected[adminID] = true
	return protected, nil
}

func (a *davACLAdapter) links(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	collectionID, err := GetReferenceIdToIdWithTransaction(a.collectionTable, collectionRef, tx)
	if err != nil {
		return nil, err
	}
	return GetObjectByWhereClauseWithTransaction(a.collectionTable+"_"+a.collectionTable+"_id_has_usergroup_usergroup_id", tx, goqu.Ex{a.collectionTable + "_id": collectionID})
}

func (a *davACLAdapter) linkPrincipal(collectionRef, groupRef daptinid.DaptinReferenceId, groupID int64, tx *sqlx.Tx) (string, error) {
	group, _, err := a.b.cruds["usergroup"].GetSingleRowByReferenceIdWithTransaction("usergroup", groupRef, nil, tx)
	if err != nil {
		return "", err
	}
	prefix := a.groupPrefix + ":" + strings.ReplaceAll(collectionRef.String(), "-", "") + ":"
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
					return "/" + a.prefix + "/" + userRef.String() + "/", nil
				}
			}
		}
	}
	return "/" + a.prefix + "/groups/" + groupRef.String() + "/", nil
}

func (a *davACLAdapter) getACL(_ context.Context, requestPath string) (*caldav.CalendarACL, error) {
	tx, err := a.b.cruds[a.collectionTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, ref, owner, err := a.collection(requestPath, tx)
	if err != nil {
		return nil, err
	}
	acl := &caldav.CalendarACL{Owner: "/" + a.prefix + "/" + owner.String() + "/"}
	acl.CanWrite, err = a.canWrite(ref, tx)
	if err != nil {
		return nil, err
	}
	links, err := a.links(ref, tx)
	if err != nil {
		return nil, err
	}
	protectedGroups, err := a.protectedGroups(owner, tx)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		groupID, ok := link["usergroup_id"].(int64)
		if !ok {
			return nil, errors.New("invalid DAV group link")
		}
		groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
		if err != nil {
			return nil, err
		}
		grant, err := davACLLinkGrant(link["permission"])
		if err != nil {
			return nil, err
		}
		privileges := a.privileges(grant)
		if len(privileges) == 0 {
			continue
		}
		principal, err := a.linkPrincipal(ref, groupRef, groupID, tx)
		if err != nil {
			return nil, err
		}
		acl.Entries = append(acl.Entries, caldav.CalendarACE{Principal: principal, Privileges: privileges, Protected: protectedGroups[groupID] || !a.representable(grant)})
	}
	return acl, tx.Commit()
}

func (a *davACLAdapter) replaceACL(_ context.Context, requestPath string, entries []caldav.CalendarACE) error {
	tx, err := a.b.cruds[a.collectionTable].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, ref, owner, err := a.collection(requestPath, tx)
	if err != nil {
		return err
	}
	share := a.share()
	collectionID, active, err := share.authorizeCollection(ref, a.b.sessionUser, tx)
	if err != nil {
		return davResourceError(err)
	}
	canWrite, err := a.canWrite(ref, tx)
	if err != nil {
		return err
	}
	if !canWrite {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("DAV ACL denied"))
	}
	old, err := a.links(ref, tx)
	if err != nil {
		return err
	}
	protectedGroups, err := a.protectedGroups(owner, tx)
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
		grant, err := davACLLinkGrant(link["permission"])
		if err != nil {
			return err
		}
		isProtected := protectedGroups[groupID] || !a.representable(grant)
		if isProtected {
			groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
			if err != nil {
				return err
			}
			protected[groupRef] = grant
			principal, err := a.linkPrincipal(ref, groupRef, groupID, tx)
			if err != nil {
				return err
			}
			protectedPrincipal[principal] = grant
		}
	}
	target := make(map[daptinid.DaptinReferenceId]bool)
	for _, entry := range entries {
		kind, principalRef, err := a.principal(entry.Principal)
		if err != nil {
			return webdav.NewACLPreconditionError("recognized-principal")
		}
		if oldGrant, ok := protectedPrincipal[entry.Principal]; ok {
			if !entry.Protected || !equalCalendarACLPrivileges(entry.Privileges, a.privileges(oldGrant)) {
				return webdav.NewACLPreconditionError("no-protected-ace-conflict")
			}
			continue
		}
		for _, privilege := range entry.Privileges {
			if privilege.Space == "DAV:" {
				switch privilege.Local {
				case "all", "write", "read-acl", "read-current-user-privilege-set":
					return webdav.NewACLPreconditionError("no-abstract")
				}
			}
		}
		grant, err := a.grant(entry.Privileges)
		if err != nil || grant == 0 {
			return webdav.NewACLPreconditionError("not-supported-privilege")
		}
		if principalRef == owner {
			return webdav.NewACLPreconditionError("no-protected-ace-conflict")
		}
		var groupRef daptinid.DaptinReferenceId
		if kind == "account" {
			allowed, err := a.actionAllowed("share_user", active, tx)
			if err != nil {
				return err
			}
			if !allowed {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("account sharing action denied"))
			}
			if entry.Protected {
				return webdav.NewACLPreconditionError("no-protected-ace-conflict")
			}
			groupRef, err = share.shareWithUser(ref, principalRef, grant, a.b.sessionUser, tx)
			if err != nil {
				return davResourceError(err)
			}
		} else {
			groupRef = principalRef
			if entry.Protected {
				return webdav.NewACLPreconditionError("no-protected-ace-conflict")
			}
			groupID, err := GetReferenceIdToIdWithTransaction("usergroup", groupRef, tx)
			if err != nil {
				return webdav.NewHTTPError(http.StatusNotFound, err)
			}
			if protectedGroups[groupID] {
				return webdav.NewACLPreconditionError("no-protected-ace-conflict")
			}
			err = share.applyAuthorized(collectionID, ref, groupRef, grant, a.b.sessionUser, active, tx)
			if err != nil {
				return davResourceError(err)
			}
		}
		if target[groupRef] {
			return webdav.NewACLPreconditionError("no-ace-conflict")
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
		if err := share.applyAuthorized(collectionID, ref, groupRef, 0, a.b.sessionUser, active, tx); err != nil {
			return davResourceError(err)
		}
	}
	return tx.Commit()
}
