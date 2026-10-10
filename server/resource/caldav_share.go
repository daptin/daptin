package resource

import (
	"errors"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
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
	grant, err := davSharePermission(fields["permission"])
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

func (a *calendarShareAction) base() *davShareAction {
	return &davShareAction{cruds: a.cruds, collectionTable: calendarCollectionTable, objectTable: calendarObjectTable, groupPrefix: "calshare"}
}

func (a *calendarShareAction) apply(collectionRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller *auth.SessionUser, tx *sqlx.Tx) error {
	return a.base().apply(collectionRef, groupRef, grant, caller, tx)
}

func (a *calendarShareAction) applyAuthorized(collectionID int64, collectionRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller, active *auth.SessionUser, tx *sqlx.Tx) error {
	return a.base().applyAuthorized(collectionID, collectionRef, groupRef, grant, caller, active, tx)
}

func (a *calendarShareAction) authorizeCollection(collectionRef daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (int64, *auth.SessionUser, error) {
	return a.base().authorizeCollection(collectionRef, caller, tx)
}

func (a *calendarShareAction) managementAccess(collectionRef daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (*auth.SessionUser, bool) {
	return a.base().managementAccess(collectionRef, caller, tx)
}

func (a *calendarShareAction) shareWithUser(collectionRef, targetRef daptinid.DaptinReferenceId, grant auth.AuthPermission, caller *auth.SessionUser, tx *sqlx.Tx) (daptinid.DaptinReferenceId, error) {
	return a.base().shareWithUser(collectionRef, targetRef, grant, caller, tx)
}

func (a *calendarShareAction) shareRequest(method, path string, user *auth.SessionUser) api2go.Request {
	return a.base().shareRequest(method, path, user)
}

func (a *calendarShareAction) setLink(table string, rowRef, groupRef daptinid.DaptinReferenceId, grant auth.AuthPermission, requestPath string, user *auth.SessionUser, tx *sqlx.Tx) error {
	return a.base().setLink(table, rowRef, groupRef, grant, requestPath, user, tx)
}
