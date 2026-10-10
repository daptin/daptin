package resource

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// davShareAction keeps group-link mutations on the existing resource path for
// the two DAV collection types. Each protocol still controls its own URLs.
type davShareAction struct {
	cruds           map[string]*DbResource
	collectionTable string
	objectTable     string
	groupPrefix     string
}

func davSharePermission(value interface{}) (auth.AuthPermission, error) {
	var bits int64
	switch number := value.(type) {
	case float64:
		if number < 0 || number > math.MaxInt64 || number != math.Trunc(number) {
			return 0, errors.New("permission must be a nonnegative integer")
		}
		bits = int64(number)
	default:
		var err error
		bits, err = ResourceRowInt64(value)
		if err != nil || bits < 0 {
			return 0, errors.New("permission must be a nonnegative integer")
		}
	}
	const allowed = auth.GroupPeek | auth.GroupRead | auth.GroupCreate | auth.GroupUpdate | auth.GroupDelete | auth.GroupExecute | auth.GroupRefer
	grant := auth.AuthPermission(bits)
	if grant & ^allowed != 0 {
		return 0, errors.New("permission may contain only existing group rights")
	}
	return grant, nil
}

func (a *davShareAction) apply(collectionRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller *auth.SessionUser, tx *sqlx.Tx) error {
	collectionID, active, err := a.authorizeCollection(collectionRef, caller, tx)
	if err != nil {
		return err
	}
	return a.applyAuthorized(collectionID, collectionRef, groupRef, grant, caller, active, tx)
}

func (a *davShareAction) applyAuthorized(collectionID int64, collectionRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller, active *auth.SessionUser, tx *sqlx.Tx) error {
	if _, err := GetReferenceIdToIdWithTransaction("usergroup", groupRef, tx); err != nil {
		return api2go.NewHTTPError(err, "usergroup not found", http.StatusNotFound)
	}
	admin := a.cruds[a.collectionTable].AdministratorGroupId
	if grant != 0 && !IsAdminWithTransaction(active, tx) {
		groupTable := a.cruds["usergroup"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "usergroup", tx)
		groupRow := GetObjectPermissionByReferenceIdWithTransaction("usergroup", groupRef, tx)
		if !groupTable.CanRefer(active.UserReferenceId, active.Groups, admin) ||
			!groupRow.CanRefer(active.UserReferenceId, active.Groups, admin) {
			return api2go.NewHTTPError(errors.New("usergroup reference denied"), "usergroup reference denied", http.StatusForbidden)
		}
	}

	objects, err := GetReferenceIdByWhereClauseWithTransaction(a.objectTable, tx, goqu.Ex{a.collectionTable + "_id": collectionID})
	if err != nil {
		return err
	}
	if err := a.setLink(a.collectionTable, collectionRef, groupRef, grant, "/action/"+a.collectionTable+"/share", caller, tx); err != nil {
		return err
	}
	for _, objectRef := range objects {
		if err := a.setLink(a.objectTable, objectRef, groupRef, grant&^auth.GroupExecute, "/action/"+a.collectionTable+"/share", caller, tx); err != nil {
			return fmt.Errorf("share object %s: %w", objectRef, err)
		}
	}
	return nil
}

func (a *davShareAction) authorizeCollection(collectionRef daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (int64, *auth.SessionUser, error) {
	collectionID, err := GetReferenceIdToIdWithTransaction(a.collectionTable, collectionRef, tx)
	if err != nil {
		return 0, nil, api2go.NewHTTPError(err, "collection not found", http.StatusNotFound)
	}
	if err := a.cruds[a.collectionTable].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": collectionRef[:]}); err != nil {
		return 0, nil, err
	}
	active, canManage := a.managementAccess(collectionRef, caller, tx)
	if !canManage {
		return 0, nil, api2go.NewHTTPError(errors.New("DAV sharing denied"), "DAV sharing denied", http.StatusForbidden)
	}
	return collectionID, active, nil
}

func (a *davShareAction) managementAccess(collectionRef daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (*auth.SessionUser, bool) {
	// Action outcomes receive a temporary administrator group. Use persisted
	// membership for both the read-only query and the sharing operation.
	groups := GetObjectGroupsByObjectIdWithTransaction(USER_ACCOUNT_TABLE_NAME, caller.UserId, tx)
	active := &auth.SessionUser{UserId: caller.UserId, UserReferenceId: caller.UserReferenceId, Groups: groups}
	collectionPermission := GetObjectPermissionByReferenceIdWithTransaction(a.collectionTable, collectionRef, tx)
	admin := a.cruds[a.collectionTable].AdministratorGroupId
	groupOnly := permission.PermissionInstance{UserGroupId: collectionPermission.UserGroupId}
	canManage := (collectionPermission.UserId == active.UserReferenceId && collectionPermission.CanExecute(active.UserReferenceId, nil, admin)) ||
		groupOnly.CanExecute(active.UserReferenceId, active.Groups, admin) || IsAdminWithTransaction(active, tx)
	return active, canManage
}

// A dedicated ordinary usergroup represents one account's access to one
// DAV collection. Its membership and resource links, rather than its name, grant
// access. The name only lets a repeated action find the same group.
func (a *davShareAction) shareWithUser(collectionRef, targetRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller *auth.SessionUser, tx *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	collectionID, active, err := a.authorizeCollection(collectionRef, caller, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	targetID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, targetRef, tx)
	if err != nil {
		return daptinid.NullReferenceId, api2go.NewHTTPError(err, "account not found", http.StatusNotFound)
	}
	admin := a.cruds[a.collectionTable].AdministratorGroupId
	if !IsAdminWithTransaction(active, tx) {
		accountTable := a.cruds[USER_ACCOUNT_TABLE_NAME].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", USER_ACCOUNT_TABLE_NAME, tx)
		accountRow := GetObjectPermissionByReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, targetRef, tx)
		if !accountTable.CanRefer(active.UserReferenceId, active.Groups, admin) || !accountRow.CanRefer(active.UserReferenceId, active.Groups, admin) {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("account reference denied"), "account reference denied", http.StatusForbidden)
		}
	}
	groupName := a.groupPrefix + ":" + strings.ReplaceAll(collectionRef.String(), "-", "") + ":" + strings.ReplaceAll(targetRef.String(), "-", "")
	groups, err := GetObjectByWhereClauseWithTransaction("usergroup", tx, goqu.Ex{"name": groupName})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	if len(groups) == 0 {
		if grant == 0 {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("account share not found"), "account share not found", http.StatusNotFound)
		}
		request := a.shareRequest(http.MethodPost, "/action/"+a.collectionTable+"/share_user", caller)
		group := api2go.NewApi2GoModelWithData("usergroup", nil, 0, nil, map[string]interface{}{"name": groupName})
		if _, err := a.cruds["usergroup"].createAfterAuthorizationWithTransaction(group, request, tx); err != nil {
			return daptinid.NullReferenceId, err
		}
		groups, err = GetObjectByWhereClauseWithTransaction("usergroup", tx, goqu.Ex{"name": groupName})
		if err != nil {
			return daptinid.NullReferenceId, err
		}
		if len(groups) != 1 {
			return daptinid.NullReferenceId, errors.New("created DAV share group is unavailable")
		}
		groupRef := daptinid.InterfaceToDIR(groups[0]["reference_id"])
		membership := api2go.NewApi2GoModelWithData("user_account_user_account_id_has_usergroup_usergroup_id", nil, 0, nil, map[string]interface{}{
			"user_account_id": targetRef.String(), "usergroup_id": groupRef.String(),
		})
		if _, err := a.cruds["user_account_user_account_id_has_usergroup_usergroup_id"].createAfterAuthorizationWithTransaction(membership, request, tx); err != nil {
			return daptinid.NullReferenceId, err
		}
	}
	if len(groups) != 1 {
		return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("ambiguous DAV share group"), "ambiguous DAV share group", http.StatusConflict)
	}
	groupID, ok := groups[0]["id"].(int64)
	if !ok {
		return daptinid.NullReferenceId, errors.New("DAV share group ID is invalid")
	}
	groupRef := daptinid.InterfaceToDIR(groups[0]["reference_id"])
	members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	if len(members) != 1 || members[0]["user_account_id"] != targetID {
		return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("DAV share membership changed"), "DAV share membership changed", http.StatusConflict)
	}
	collectionLinks, err := GetObjectByWhereClauseWithTransaction(a.collectionTable+"_"+a.collectionTable+"_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	for _, link := range collectionLinks {
		if link[a.collectionTable+"_id"] != collectionID {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("DAV share group belongs to another collection"), "DAV share group belongs to another collection", http.StatusConflict)
		}
	}
	if err := a.applyAuthorized(collectionID, collectionRef, groupRef, grant, caller, active, tx); err != nil {
		return daptinid.NullReferenceId, err
	}
	return groupRef, nil
}

func (a *davShareAction) shareRequest(method, path string, user *auth.SessionUser) api2go.Request {
	httpRequest := (&http.Request{Method: method, URL: &url.URL{Path: path}}).WithContext(context.WithValue(context.Background(), "user", user))
	return api2go.Request{PlainRequest: httpRequest}
}

func (a *davShareAction) setLink(table string, rowRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, requestPath string, user *auth.SessionUser, tx *sqlx.Tx) error {
	rowID, err := GetReferenceIdToIdWithTransaction(table, rowRef, tx)
	if err != nil {
		return err
	}
	groupID, err := GetReferenceIdToIdWithTransaction("usergroup", groupRef, tx)
	if err != nil {
		return err
	}
	joinTable := table + "_" + table + "_id_has_usergroup_usergroup_id"
	links, err := GetObjectByWhereClauseWithTransaction(joinTable, tx, goqu.Ex{table + "_id": rowID, "usergroup_id": groupID})
	if err != nil {
		return err
	}
	requestURL := &url.URL{Path: requestPath}
	request := func(method string) api2go.Request {
		httpRequest := (&http.Request{Method: method, URL: requestURL}).WithContext(context.WithValue(context.Background(), "user", user))
		return api2go.Request{PlainRequest: httpRequest}
	}
	crud := a.cruds[joinTable]
	if crud == nil {
		return fmt.Errorf("DAV relationship resource %s is unavailable", joinTable)
	}
	if grant == 0 {
		for _, link := range links {
			if _, err := crud.deleteAfterAuthorizationWithTransaction(daptinid.InterfaceToDIR(link["reference_id"]), request(http.MethodDelete), tx); err != nil {
				return err
			}
		}
		return nil
	}
	if len(links) == 0 {
		model := api2go.NewApi2GoModelWithData(joinTable, nil, 0, nil, map[string]interface{}{
			table + "_id": rowRef.String(), "usergroup_id": groupRef.String(),
		})
		if _, err := crud.createAfterAuthorizationWithTransaction(model, request(http.MethodPost), tx); err != nil {
			return err
		}
		links, err = GetObjectByWhereClauseWithTransaction(joinTable, tx, goqu.Ex{table + "_id": rowID, "usergroup_id": groupID})
		if err != nil {
			return err
		}
	}
	if len(links) != 1 {
		return fmt.Errorf("expected one DAV group link, found %d", len(links))
	}
	model := api2go.NewApi2GoModelWithData(joinTable, nil, 0, nil, links[0])
	model.SetAttributes(map[string]interface{}{"permission": int64(grant)})
	_, err = crud.updateAfterAuthorizationWithTransaction(model, request(http.MethodPatch), tx)
	return err
}
