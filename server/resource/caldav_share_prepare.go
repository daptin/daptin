package resource

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// Creator rights on an event survive removal of its calendar share. This
// action is opt-in: administrators still choose the calendar row default.
const calendarCreatorRights = auth.UserCRUD | auth.UserExecute

type calendarPrepareSharingAction struct {
	share *calendarShareAction
}

func NewCalendarPrepareSharingAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &calendarPrepareSharingAction{share: &calendarShareAction{cruds: cruds}}
}

func (*calendarPrepareSharingAction) Name() string { return "calendar.prepare_sharing" }

func (a *calendarPrepareSharingAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar preparation requires an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || caller.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	if collectionRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid reference ID"), "invalid reference ID", http.StatusBadRequest)}
	}
	collectionID, active, err := a.share.authorizeCollection(collectionRef, caller, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	owner := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx).UserId
	if owner == daptinid.NullReferenceId || (owner != active.UserReferenceId && !IsAdminWithTransaction(active, tx)) {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("collection owner required"), "collection owner required", http.StatusForbidden)}
	}
	eventCRUD := a.share.cruds[calendarObjectTable]
	if auth.AuthPermission(eventCRUD.TableInfo().DefaultPermission)&(calendarCreatorRights|auth.GuestCRUD) != 0 {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar row default grants independent access"), "configure a revocable calendar row default first", http.StatusConflict)}
	}
	// shareWithUser uses the same locked collection and action transaction. It
	// creates the owner's ordinary one-account group and grants existing rows.
	ownerGroup, err := a.share.shareWithUser(collectionRef, owner, auth.GroupCRUD|auth.GroupExecute, caller, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	updated := 0
	err = forEachCalendarEvent(collectionID, tx, func(row map[string]interface{}) error {
		ref := daptinid.InterfaceToDIR(row["reference_id"])
		bits, err := ResourceRowInt64(row["permission"])
		if err != nil || ref == daptinid.NullReferenceId {
			return errors.New("invalid calendar event permission or reference ID")
		}
		if auth.AuthPermission(bits)&auth.GuestCRUD != 0 {
			return api2go.NewHTTPError(errors.New("event grants public access"), "event has independent public access", http.StatusConflict)
		}
		permission := auth.AuthPermission(bits) &^ calendarCreatorRights
		if permission == auth.AuthPermission(bits) {
			return nil
		}
		stored, _, err := eventCRUD.GetSingleRowByReferenceIdWithTransaction(calendarObjectTable, ref, nil, tx)
		if err != nil {
			return err
		}
		model := api2go.NewApi2GoModelWithData(calendarObjectTable, nil, 0, nil, stored)
		model.SetAttributes(map[string]interface{}{"permission": int64(permission)})
		_, err = eventCRUD.updateAfterAuthorizationWithTransaction(model,
			a.share.shareRequest(http.MethodPatch, "/action/collection/prepare_sharing", caller), tx)
		if err == nil {
			updated++
		}
		return err
	})
	if err != nil {
		return nil, nil, []error{err}
	}
	ready, err := a.share.revocableEditSharingReady(collectionRef, collectionID, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	if !ready {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar owner access is unavailable"), "calendar sharing is not ready", http.StatusConflict)}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.prepare_sharing", map[string]interface{}{
		"collection_id": collectionRef.String(), "owner_group_id": ownerGroup.String(), "events_updated": updated,
	})}, nil
}

// Iteration stays bounded in memory and ordered by the public reference ID.
func forEachCalendarEvent(collectionID int64, tx *sqlx.Tx, visit func(map[string]interface{}) error) error {
	const pageSize = 512
	cursor := daptinid.NullReferenceId
	for {
		filters := []goqu.Ex{{"collection_id": collectionID}}
		if cursor != daptinid.NullReferenceId {
			filters = append(filters, goqu.Ex{"reference_id": goqu.Op{"gt": cursor[:]}})
		}
		rows, err := GetLimitedOrderedRowsWithTransaction(calendarObjectTable,
			[]string{"id", "reference_id", "permission"}, "reference_id", tx, pageSize, filters...)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := visit(row); err != nil {
				return err
			}
			cursor = daptinid.InterfaceToDIR(row["reference_id"])
			if cursor == daptinid.NullReferenceId {
				return fmt.Errorf("calendar event has no reference ID")
			}
		}
		if len(rows) < pageSize {
			return nil
		}
	}
}

// The owner-only group is the persisted grant prepared calendars need for
// events created by another account. Its collection link and membership, not
// its name alone, identify the protected share.
func (a *calendarShareAction) protectedOwnerShareGroup(collectionRef daptinid.DaptinReferenceId, collectionID int64, tx *sqlx.Tx) (int64, daptinid.DaptinReferenceId, error) {
	defaultPermission := auth.AuthPermission(a.cruds[calendarObjectTable].TableInfo().DefaultPermission)
	if defaultPermission&(calendarCreatorRights|auth.GuestCRUD) != 0 {
		return 0, daptinid.NullReferenceId, nil
	}
	owner := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx).UserId
	if owner == daptinid.NullReferenceId {
		return 0, owner, nil
	}
	groups, err := GetObjectByWhereClauseWithTransaction("usergroup", tx,
		goqu.Ex{"name": davAccountShareGroupName("calshare", collectionRef, owner)})
	if err != nil || len(groups) == 0 {
		return 0, owner, err
	}
	if len(groups) != 1 {
		return 0, owner, errors.New("ambiguous calendar owner share group")
	}
	groupID, ok := groups[0]["id"].(int64)
	if !ok {
		return 0, owner, errors.New("invalid calendar owner share group ID")
	}
	ownerID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, owner, tx)
	if err != nil {
		return 0, owner, err
	}
	members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx,
		goqu.Ex{"usergroup_id": groupID})
	if err != nil {
		return 0, owner, err
	}
	if len(members) != 1 || members[0]["user_account_id"] != ownerID {
		return 0, owner, nil
	}
	links, err := GetObjectByWhereClauseWithTransaction("collection_collection_id_has_usergroup_usergroup_id", tx,
		goqu.Ex{"collection_id": collectionID, "usergroup_id": groupID})
	if err != nil || len(links) == 0 {
		return 0, owner, err
	}
	if len(links) != 1 {
		return 0, owner, errors.New("ambiguous calendar owner share link")
	}
	return groupID, owner, nil
}

// This reports whether creator permissions can outlive an account share. Other
// independently granted groups remain independent Daptin access authority.
func (a *calendarShareAction) revocableEditSharingReady(collectionRef daptinid.DaptinReferenceId, collectionID int64, tx *sqlx.Tx) (bool, error) {
	eventCRUD := a.cruds[calendarObjectTable]
	if auth.AuthPermission(eventCRUD.TableInfo().DefaultPermission)&(calendarCreatorRights|auth.GuestCRUD) != 0 {
		return false, nil
	}
	owner := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx).UserId
	if owner == daptinid.NullReferenceId {
		return false, nil
	}
	groupName := davAccountShareGroupName("calshare", collectionRef, owner)
	groups, err := GetObjectByWhereClauseWithTransaction("usergroup", tx, goqu.Ex{"name": groupName})
	if err != nil || len(groups) != 1 {
		return false, err
	}
	groupID, ok := groups[0]["id"].(int64)
	if !ok {
		return false, errors.New("owner share group has invalid ID")
	}
	ownerID, err := GetReferenceIdToIdWithTransaction(USER_ACCOUNT_TABLE_NAME, owner, tx)
	if err != nil {
		return false, err
	}
	members, err := GetObjectByWhereClauseWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
	if err != nil || len(members) != 1 || members[0]["user_account_id"] != ownerID {
		return false, err
	}
	ownerGroups := GetObjectGroupsByObjectIdWithTransaction(USER_ACCOUNT_TABLE_NAME, ownerID, tx)
	collectionCRUD := a.cruds[calendarCollectionTable]
	collectionTable := collectionCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	collection := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	canRead, _ := calendarDiscoveryAccess(owner, ownerGroups, collectionTable, collection, eventTable,
		collectionCRUD.AdministratorGroupId, eventCRUD.AdministratorGroupId)
	if !canRead {
		return false, nil
	}
	collectionLinks, err := GetObjectByWhereClauseWithTransaction("collection_collection_id_has_usergroup_usergroup_id", tx,
		goqu.Ex{"collection_id": collectionID, "usergroup_id": groupID})
	if err != nil || len(collectionLinks) != 1 {
		return false, err
	}
	grant, err := ResourceRowInt64(collectionLinks[0]["permission"])
	if err != nil || auth.AuthPermission(grant)&auth.GroupCRUD != auth.GroupCRUD {
		return false, err
	}
	links, err := GetObjectByWhereClauseWithTransaction("calendar_calendar_id_has_usergroup_usergroup_id", tx, goqu.Ex{"usergroup_id": groupID})
	if err != nil {
		return false, err
	}
	granted := make(map[int64]bool, len(links))
	for _, link := range links {
		id, ok := link["calendar_id"].(int64)
		bits, bitErr := ResourceRowInt64(link["permission"])
		if ok && bitErr == nil && auth.AuthPermission(bits)&auth.GroupCRUD == auth.GroupCRUD {
			granted[id] = true
		}
	}
	ready := true
	err = forEachCalendarEvent(collectionID, tx, func(row map[string]interface{}) error {
		bits, bitErr := ResourceRowInt64(row["permission"])
		id, ok := row["id"].(int64)
		if bitErr != nil || !ok {
			return errors.New("invalid calendar event permission or ID")
		}
		if auth.AuthPermission(bits)&(calendarCreatorRights|auth.GuestCRUD) != 0 || !granted[id] {
			ready = false
		}
		return nil
	})
	return ready, err
}
