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
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// calendarShareAction applies ordinary group links to a collection and its
// existing events in the action transaction. Link permissions remain the
// resource authority for DAV and JSON:API access.
type calendarShareAction struct {
	cruds map[string]*DbResource
}

func NewCalendarShareAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &calendarShareAction{cruds: cruds}
}

func (*calendarShareAction) Name() string { return "calendar.share" }

type calendarShareCapabilitiesAction struct {
	share *calendarShareAction
}

func NewCalendarShareCapabilitiesAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &calendarShareCapabilitiesAction{share: &calendarShareAction{cruds: cruds}}
}

func (*calendarShareCapabilitiesAction) Name() string { return "calendar.share_capabilities" }

type calendarEventCapabilitiesAction struct {
	share *calendarShareAction
}

func NewCalendarEventCapabilitiesAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &calendarEventCapabilitiesAction{share: &calendarShareAction{cruds: cruds}}
}

func (*calendarEventCapabilitiesAction) Name() string { return "calendar.event_capabilities" }

func (a *calendarEventCapabilitiesAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar capabilities require an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || caller.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	eventRef := daptinid.InterfaceToDIR(fields["event_id"])
	if collectionRef == daptinid.NullReferenceId || eventRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid reference ID"), "invalid reference ID", http.StatusBadRequest)}
	}
	active, _ := a.share.managementAccess(collectionRef, caller, tx)
	collectionCRUD := a.share.cruds[calendarCollectionTable]
	eventCRUD := a.share.cruds[calendarObjectTable]
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "collection not found", http.StatusNotFound)}
	}
	admin := collectionCRUD.AdministratorGroupId
	user, groups := active.UserReferenceId, active.Groups
	collectionTable := collectionCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	collection := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if !collectionTable.CanRead(user, groups, admin) || !collection.CanRead(user, groups, admin) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar access denied"), "calendar access denied", http.StatusForbidden)}
	}
	events, err := GetReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx,
		goqu.Ex{"collection_id": collectionID, "reference_id": eventRef[:]})
	if err != nil {
		return nil, nil, []error{err}
	}
	if len(events) != 1 {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("event not in calendar"), "event not in calendar", http.StatusNotFound)}
	}
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	event := GetObjectPermissionByReferenceIdWithTransaction(calendarObjectTable, eventRef, tx)
	if !eventTable.CanRead(user, groups, admin) || !event.CanRead(user, groups, admin) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("event detail access denied"), "event detail access denied", http.StatusForbidden)}
	}
	canEditCollection := collection.CanUpdate(user, groups, admin)
	return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.event_capabilities", map[string]interface{}{
		"collection_id": collectionRef.String(), "event_id": eventRef.String(),
		"can_update": canEditCollection && eventTable.CanUpdate(user, groups, admin) && event.CanUpdate(user, groups, admin),
		"can_delete": canEditCollection && eventTable.CanDelete(user, groups, admin) && event.CanDelete(user, groups, admin),
	})}, nil
}

func (a *calendarShareCapabilitiesAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar capabilities require an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || caller.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	if collectionRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid reference ID"), "invalid reference ID", http.StatusBadRequest)}
	}
	if _, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx); err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "collection not found", http.StatusNotFound)}
	}
	active, canManage := a.share.managementAccess(collectionRef, caller, tx)
	admin := a.share.cruds[calendarCollectionTable].AdministratorGroupId
	collectionTable := a.share.cruds[calendarCollectionTable].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	collection := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	canDiscover := (collectionTable.CanRead(active.UserReferenceId, active.Groups, admin) || collectionTable.CanPeek(active.UserReferenceId, active.Groups, admin)) &&
		(collection.CanRead(active.UserReferenceId, active.Groups, admin) || collection.CanPeek(active.UserReferenceId, active.Groups, admin))
	if !canDiscover && !canManage {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar access denied"), "calendar access denied", http.StatusForbidden)}
	}
	allowed := func(name string) (bool, error) {
		if !canManage {
			return false, nil
		}
		action, err := a.share.cruds[calendarCollectionTable].GetActionByName(calendarCollectionTable, name, tx)
		if err != nil {
			return false, err
		}
		permission := GetObjectPermissionByReferenceIdWithTransaction("action", action.ReferenceId, tx)
		return IsAdminWithTransaction(active, tx) || permission.CanExecute(active.UserReferenceId, active.Groups, admin), nil
	}
	canShareGroup, err := allowed("share")
	if err != nil {
		return nil, nil, []error{err}
	}
	canShareUser, err := allowed("share_user")
	if err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.share_capabilities", map[string]interface{}{
		"collection_id": collectionRef.String(), "can_share_group": canShareGroup, "can_share_user": canShareUser,
	})}, nil
}

func (a *calendarShareAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar sharing requires an action transaction")}
	}
	user, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || user == nil || user.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	if collectionRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid reference ID"), "invalid reference ID", http.StatusBadRequest)}
	}
	grant, err := calendarSharePermission(fields["permission"])
	if err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "invalid group permission", http.StatusBadRequest)}
	}
	groupRef := daptinid.InterfaceToDIR(fields["usergroup_id"])
	if target, present := fields["target_user_id"]; present {
		if groupRef != daptinid.NullReferenceId {
			return nil, nil, []error{api2go.NewHTTPError(errors.New("choose an account or group"), "choose an account or group", http.StatusBadRequest)}
		}
		targetRef := daptinid.InterfaceToDIR(target)
		if targetRef == daptinid.NullReferenceId {
			return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid account reference ID"), "invalid account reference ID", http.StatusBadRequest)}
		}
		var err error
		groupRef, err = a.shareWithUser(collectionRef, targetRef, grant, user, tx)
		if err != nil {
			return nil, nil, []error{err}
		}
		return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.share", map[string]interface{}{
			"collection_id": collectionRef.String(), "user_account_id": targetRef.String(), "usergroup_id": groupRef.String(), "permission": grant,
		})}, nil
	}
	if groupRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid group reference ID"), "invalid group reference ID", http.StatusBadRequest)}
	}
	if err := a.apply(collectionRef, groupRef, grant, user, tx); err != nil {
		return nil, nil, []error{err}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.share", map[string]interface{}{
		"collection_id": collectionRef.String(), "usergroup_id": groupRef.String(), "permission": grant,
	})}, nil
}

func calendarSharePermission(value interface{}) (auth.AuthPermission, error) {
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

func (a *calendarShareAction) apply(collectionRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller *auth.SessionUser, tx *sqlx.Tx) error {
	collectionID, active, err := a.authorizeCollection(collectionRef, caller, tx)
	if err != nil {
		return err
	}
	return a.applyAuthorized(collectionID, collectionRef, groupRef, grant, caller, active, tx)
}

func (a *calendarShareAction) applyAuthorized(collectionID int64, collectionRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller, active *auth.SessionUser, tx *sqlx.Tx) error {
	if _, err := GetReferenceIdToIdWithTransaction("usergroup", groupRef, tx); err != nil {
		return api2go.NewHTTPError(err, "usergroup not found", http.StatusNotFound)
	}
	admin := a.cruds[calendarCollectionTable].AdministratorGroupId
	if grant != 0 && !IsAdminWithTransaction(active, tx) {
		groupTable := a.cruds["usergroup"].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "usergroup", tx)
		groupRow := GetObjectPermissionByReferenceIdWithTransaction("usergroup", groupRef, tx)
		if !groupTable.CanRefer(active.UserReferenceId, active.Groups, admin) ||
			!groupRow.CanRefer(active.UserReferenceId, active.Groups, admin) {
			return api2go.NewHTTPError(errors.New("usergroup reference denied"), "usergroup reference denied", http.StatusForbidden)
		}
	}

	events, err := GetReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx, goqu.Ex{"collection_id": collectionID})
	if err != nil {
		return err
	}
	if err := a.setLink(calendarCollectionTable, collectionRef, groupRef, grant, "/action/collection/share", caller, tx); err != nil {
		return err
	}
	for _, eventRef := range events {
		if err := a.setLink(calendarObjectTable, eventRef, groupRef, grant&^auth.GroupExecute, "/action/collection/share", caller, tx); err != nil {
			return fmt.Errorf("share event %s: %w", eventRef, err)
		}
	}
	return nil
}

func (a *calendarShareAction) authorizeCollection(collectionRef daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (int64, *auth.SessionUser, error) {
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return 0, nil, api2go.NewHTTPError(err, "collection not found", http.StatusNotFound)
	}
	if err := a.cruds[calendarCollectionTable].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": collectionRef[:]}); err != nil {
		return 0, nil, err
	}
	active, canManage := a.managementAccess(collectionRef, caller, tx)
	if !canManage {
		return 0, nil, api2go.NewHTTPError(errors.New("calendar sharing denied"), "calendar sharing denied", http.StatusForbidden)
	}
	return collectionID, active, nil
}

func (a *calendarShareAction) managementAccess(collectionRef daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (*auth.SessionUser, bool) {
	// Action outcomes receive a temporary administrator group. Use persisted
	// membership for both the read-only query and the sharing operation.
	groups := GetObjectGroupsByObjectIdWithTransaction(USER_ACCOUNT_TABLE_NAME, caller.UserId, tx)
	active := &auth.SessionUser{UserId: caller.UserId, UserReferenceId: caller.UserReferenceId, Groups: groups}
	collectionPermission := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	admin := a.cruds[calendarCollectionTable].AdministratorGroupId
	groupOnly := permission.PermissionInstance{UserGroupId: collectionPermission.UserGroupId}
	canManage := (collectionPermission.UserId == active.UserReferenceId && collectionPermission.CanExecute(active.UserReferenceId, nil, admin)) ||
		groupOnly.CanExecute(active.UserReferenceId, active.Groups, admin) || IsAdminWithTransaction(active, tx)
	return active, canManage
}

// A dedicated ordinary usergroup represents one account's access to one
// calendar. Its membership and resource links, rather than its name, grant
// access. The name only lets a repeated action find the same group.
func (a *calendarShareAction) shareWithUser(collectionRef, targetRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller *auth.SessionUser, tx *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	collectionID, active, err := a.authorizeCollection(collectionRef, caller, tx)
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	targetID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, targetRef, tx)
	if err != nil {
		return daptinid.NullReferenceId, api2go.NewHTTPError(err, "account not found", http.StatusNotFound)
	}
	admin := a.cruds[calendarCollectionTable].AdministratorGroupId
	if !IsAdminWithTransaction(active, tx) {
		accountTable := a.cruds[USER_ACCOUNT_TABLE_NAME].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", USER_ACCOUNT_TABLE_NAME, tx)
		accountRow := GetObjectPermissionByReferenceIdWithTransaction(USER_ACCOUNT_TABLE_NAME, targetRef, tx)
		if !accountTable.CanRefer(active.UserReferenceId, active.Groups, admin) || !accountRow.CanRefer(active.UserReferenceId, active.Groups, admin) {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("account reference denied"), "account reference denied", http.StatusForbidden)
		}
	}
	groupName := "calshare:" + strings.ReplaceAll(collectionRef.String(), "-", "") + ":" + strings.ReplaceAll(targetRef.String(), "-", "")
	groups, err := GetObjectByWhereClauseWithTransaction("usergroup", tx, goqu.Ex{"name": groupName})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	if len(groups) == 0 {
		if grant == 0 {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("account share not found"), "account share not found", http.StatusNotFound)
		}
		request := a.shareRequest(http.MethodPost, "/action/collection/share_user", caller)
		group := api2go.NewApi2GoModelWithData("usergroup", nil, 0, nil, map[string]interface{}{"name": groupName})
		if _, err := a.cruds["usergroup"].createAfterAuthorizationWithTransaction(group, request, tx); err != nil {
			return daptinid.NullReferenceId, err
		}
		groups, err = GetObjectByWhereClauseWithTransaction("usergroup", tx, goqu.Ex{"name": groupName})
		if err != nil {
			return daptinid.NullReferenceId, err
		}
		if len(groups) != 1 {
			return daptinid.NullReferenceId, errors.New("created calendar share group is unavailable")
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
		return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("ambiguous calendar share group"), "ambiguous calendar share group", http.StatusConflict)
	}
	groupID, ok := groups[0]["id"].(int64)
	if !ok {
		return daptinid.NullReferenceId, errors.New("calendar share group ID is invalid")
	}
	groupRef := daptinid.InterfaceToDIR(groups[0]["reference_id"])
	members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	if len(members) != 1 || members[0]["user_account_id"] != targetID {
		return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("calendar share membership changed"), "calendar share membership changed", http.StatusConflict)
	}
	collectionLinks, err := GetObjectByWhereClauseWithTransaction("collection_collection_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
	if err != nil {
		return daptinid.NullReferenceId, err
	}
	for _, link := range collectionLinks {
		if link["collection_id"] != collectionID {
			return daptinid.NullReferenceId, api2go.NewHTTPError(errors.New("calendar share group belongs to another collection"), "calendar share group belongs to another collection", http.StatusConflict)
		}
	}
	if err := a.applyAuthorized(collectionID, collectionRef, groupRef, grant, caller, active, tx); err != nil {
		return daptinid.NullReferenceId, err
	}
	return groupRef, nil
}

func (a *calendarShareAction) shareRequest(method, path string, user *auth.SessionUser) api2go.Request {
	httpRequest := (&http.Request{Method: method, URL: &url.URL{Path: path}}).WithContext(context.WithValue(context.Background(), "user", user))
	return api2go.Request{PlainRequest: httpRequest}
}

func (a *calendarShareAction) setLink(table string, rowRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, requestPath string, user *auth.SessionUser, tx *sqlx.Tx) error {
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
		return fmt.Errorf("calendar relationship resource %s is unavailable", joinTable)
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
		return fmt.Errorf("expected one calendar group link, found %d", len(links))
	}
	model := api2go.NewApi2GoModelWithData(joinTable, nil, 0, nil, links[0])
	model.SetAttributes(map[string]interface{}{"permission": int64(grant)})
	_, err = crud.updateAfterAuthorizationWithTransaction(model, request(http.MethodPatch), tx)
	return err
}
