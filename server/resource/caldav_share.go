package resource

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"

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

func (a *calendarShareAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar sharing requires an action transaction")}
	}
	user, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || user == nil || user.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	groupRef := daptinid.InterfaceToDIR(fields["usergroup_id"])
	if collectionRef == daptinid.NullReferenceId || groupRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid reference ID"), "invalid reference ID", http.StatusBadRequest)}
	}
	grant, err := calendarSharePermission(fields["permission"])
	if err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "invalid group permission", http.StatusBadRequest)}
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
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return api2go.NewHTTPError(err, "collection not found", http.StatusNotFound)
	}
	if err := a.cruds[calendarCollectionTable].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": collectionRef[:]}); err != nil {
		return err
	}
	// Action outcomes receive a temporary administrator group. Rebuild membership
	// from persisted links before deciding whether this account may manage shares.
	groups := GetObjectGroupsByObjectIdWithTransaction(USER_ACCOUNT_TABLE_NAME, caller.UserId, tx)
	active := &auth.SessionUser{UserId: caller.UserId, UserReferenceId: caller.UserReferenceId, Groups: groups}
	collectionPermission := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	admin := a.cruds[calendarCollectionTable].AdministratorGroupId
	groupOnly := permission.PermissionInstance{UserGroupId: collectionPermission.UserGroupId}
	canManage := (collectionPermission.UserId == active.UserReferenceId && collectionPermission.CanExecute(active.UserReferenceId, nil, admin)) ||
		groupOnly.CanExecute(active.UserReferenceId, active.Groups, admin) || IsAdminWithTransaction(active, tx)
	if !canManage {
		return api2go.NewHTTPError(errors.New("calendar sharing denied"), "calendar sharing denied", http.StatusForbidden)
	}
	if _, err := GetReferenceIdToIdWithTransaction("usergroup", groupRef, tx); err != nil {
		return api2go.NewHTTPError(err, "usergroup not found", http.StatusNotFound)
	}
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
