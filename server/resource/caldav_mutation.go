package resource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/caldav"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// PatchCalendarProperties changes only metadata represented by the collection
// resource. Its name and URL remain stable when the display name changes.
func (b *DaptinDAVBackend) PatchCalendarProperties(_ context.Context, requestPath string, changes []caldav.CalendarPropertyUpdate) error {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return err
	}
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return err
	}
	if !b.calendarEditAllowed(collection, tx, false) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar metadata update denied"))
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return err
	}
	attrs := make(map[string]interface{}, len(changes))
	for _, change := range changes {
		value := change.Value
		if change.Remove {
			value = ""
		}
		switch {
		case change.Name.Space == "DAV:" && change.Name.Local == "displayname":
			attrs["display_name"] = value
		case change.Name.Space == "urn:ietf:params:xml:ns:caldav" && change.Name.Local == "calendar-description":
			attrs["description"] = value
		default:
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("protected calendar property"))
		}
	}
	if len(attrs) == 0 {
		return webdav.NewHTTPError(http.StatusBadRequest, errors.New("no calendar properties supplied"))
	}
	if err := b.calendarUpdate(calendarCollectionTable, path.Clean(requestPath), collection, attrs, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// deleteCalendarCollection removes its objects and queues their scheduling
// changes before the collection disappears. All writes share the DAV transaction.
func (b *DaptinDAVBackend) deleteCalendarCollection(requestPath string, collection map[string]interface{}, tx *sqlx.Tx) error {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	crud := b.cruds[calendarCollectionTable]
	table := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	row := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId
	if !table.CanDelete(user, groups, admin) || !row.CanDelete(user, groups, admin) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar deletion denied"))
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return err
	}
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return err
	}
	eventCRUD := b.cruds[calendarObjectTable]
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	if !eventTable.CanRead(user, groups, admin) || !eventTable.CanDelete(user, groups, admin) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar object deletion denied"))
	}
	const batchSize = 100
	for {
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx, batchSize, goqu.Ex{"collection_id": collectionID})
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			break
		}
		for _, ref := range refs {
			protected, err := b.bookingEventProtected(ref, tx)
			if err != nil {
				return err
			}
			if protected {
				return webdav.NewHTTPError(http.StatusConflict, errors.New("calendar contains booking events"))
			}
			eventPermission := GetObjectPermissionByReferenceIdWithTransaction(calendarObjectTable, ref, tx)
			if !eventPermission.CanRead(user, groups, admin) || !eventPermission.CanDelete(user, groups, admin) {
				return webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar object deletion denied"))
			}
			event, _, err := eventCRUD.GetSingleRowByReferenceIdWithTransaction(calendarObjectTable, ref, nil, tx)
			if err != nil {
				return err
			}
			previous, err := b.contentBytes(calendarObjectTable, event)
			if err != nil {
				return err
			}
			eventPath := StringOrEmpty(event["rpath"])
			if err := b.calendarDelete(calendarObjectTable, eventPath, event, tx); err != nil {
				return err
			}
			if err := b.scheduleCalendarChange(collection, ref, previous, nil, nil, tx); err != nil {
				return err
			}
		}
	}
	// Calendar mail remains as delivery history. Its optional collection link
	// must be cleared before deleting the collection on databases with FKs.
	for {
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("cal_mail", tx, batchSize, goqu.Ex{"collection_id": collectionID})
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			break
		}
		for _, ref := range refs {
			model := api2go.NewApi2GoModelWithData("cal_mail", nil, 0, nil, map[string]interface{}{
				"reference_id": ref.String(), "collection_id": nil,
			})
			if _, err := b.cruds["cal_mail"].updateAfterAuthorizationWithTransaction(model,
				b.request(http.MethodPatch, "/api/cal_mail/"+ref.String()), tx); err != nil {
				return davResourceError(err)
			}
		}
	}
	if err := b.deleteDAVSyncState(collectionRef, tx); err != nil {
		return err
	}
	if err := b.deleteDAVLogState(collectionRef, tx); err != nil {
		return err
	}
	if err := b.deleteDAVClock(collectionRef, tx); err != nil {
		return err
	}
	return b.calendarDelete(calendarCollectionTable, requestPath, collection, tx)
}

func (b *DaptinDAVBackend) CopyCalendarObject(_ context.Context, sourcePath, destinationPath string, overwrite bool) (bool, error) {
	return b.transferCalendarObject(sourcePath, destinationPath, overwrite, false)
}

func (b *DaptinDAVBackend) MoveCalendarObject(_ context.Context, sourcePath, destinationPath string, overwrite bool) (bool, error) {
	return b.transferCalendarObject(sourcePath, destinationPath, overwrite, true)
}

func (b *DaptinDAVBackend) transferCalendarObject(sourcePath, destinationPath string, overwrite, move bool) (bool, error) {
	sourceOwner, sourceName, err := b.calendarPath(sourcePath, true)
	if err != nil {
		return false, err
	}
	destinationOwner, destinationName, err := b.calendarPath(destinationPath, true)
	if err != nil {
		return false, err
	}
	if path.Clean(sourcePath) == path.Clean(destinationPath) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar source and destination are the same"))
	}
	tx, err := b.cruds[calendarObjectTable].Connection().Beginx()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	sourceCollection, err := b.calendarCollection(sourceOwner, sourceName, sourcePath, tx)
	if err != nil {
		return false, err
	}
	destinationCollection, err := b.calendarCollection(destinationOwner, destinationName, destinationPath, tx)
	if err != nil {
		return false, err
	}
	sourceCollectionRef := daptinid.InterfaceToDIR(sourceCollection["reference_id"])
	destinationCollectionRef := daptinid.InterfaceToDIR(destinationCollection["reference_id"])
	first, second := sourceCollection, destinationCollection
	if sourceCollectionRef.String() > destinationCollectionRef.String() {
		first, second = second, first
	}
	if err := b.lockCalendarCollection(first, tx); err != nil {
		return false, err
	}
	if sourceCollectionRef != destinationCollectionRef {
		if err := b.lockCalendarCollection(second, tx); err != nil {
			return false, err
		}
	}
	source, err := b.calendarObjectRow(sourcePath, sourceCollection, tx)
	if err != nil {
		return false, err
	}
	sourceRef := daptinid.InterfaceToDIR(source["reference_id"])
	protected, err := b.bookingEventProtected(sourceRef, tx)
	if err != nil {
		return false, err
	}
	if protected {
		return false, webdav.NewHTTPError(http.StatusConflict, errors.New("booking events are managed through booking actions"))
	}
	content, err := b.contentBytes(calendarObjectTable, source)
	if err != nil {
		return false, err
	}
	calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
	if err != nil {
		return false, err
	}
	uid, organizer := itipCalendarIdentity(calendar)
	destination, findErr := b.calendarObjectRow(destinationPath, destinationCollection, tx)
	exists := findErr == nil
	if findErr != nil && !errors.Is(findErr, errDAVNotFound) {
		return false, findErr
	}
	if exists && !overwrite {
		return false, webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar destination exists"))
	}
	if exists {
		protected, err := b.bookingEventProtected(daptinid.InterfaceToDIR(destination["reference_id"]), tx)
		if err != nil {
			return false, err
		}
		if protected {
			return false, webdav.NewHTTPError(http.StatusConflict, errors.New("booking events are managed through booking actions"))
		}
	}
	if !b.calendarEditAllowed(destinationCollection, tx, !exists) ||
		move && (!b.calendarEditAllowed(destinationCollection, tx, true) || !b.calendarEditAllowed(sourceCollection, tx, false)) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar transfer denied"))
	}
	if move {
		crud := b.cruds[calendarObjectTable]
		table := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
		if !table.CanCreate(b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId) {
			return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar destination create denied"))
		}
	}
	if move && (!b.calendarEventAllowed(sourceRef, tx, http.MethodDelete) || !b.calendarEventAllowed(sourceRef, tx, http.MethodPatch)) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar source transfer denied"))
	}
	if exists {
		method := http.MethodPatch
		if move {
			method = http.MethodDelete
		}
		if !b.calendarEventAllowed(daptinid.InterfaceToDIR(destination["reference_id"]), tx, method) {
			return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar destination overwrite denied"))
		}
	}
	if uid != "" {
		collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, destinationCollectionRef, tx)
		if err != nil {
			return false, err
		}
		refs, err := GetReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx,
			goqu.Ex{"collection_id": collectionID, "uid": uid})
		if err != nil {
			return false, err
		}
		for _, ref := range refs {
			if exists && ref == daptinid.InterfaceToDIR(destination["reference_id"]) || move && ref == sourceRef {
				continue
			}
			return false, webdav.NewHTTPError(http.StatusConflict, errors.New("calendar UID already exists in destination"))
		}
	}
	tag := ""
	if uid != "" && organizer != "" && daptinid.InterfaceToDIR(destinationCollection["scheduling_mail_account_id"]) != daptinid.NullReferenceId {
		tag = uuid.NewString()
	}
	attrs := map[string]interface{}{
		"rpath": path.Clean(destinationPath), "collection_id": destinationCollectionRef.String(),
		"content": b.contentValue(calendarObjectTable, destinationPath, ical.MIMEType, content),
		"uid":     uid, "organizer_address": organizer, "schedule_tag": tag,
	}
	if move {
		if exists {
			if err := b.calendarDelete(calendarObjectTable, destinationPath, destination, tx); err != nil {
				return false, err
			}
		}
		if err := b.calendarUpdate(calendarObjectTable, sourcePath, source, attrs, tx); err != nil {
			return false, err
		}
		if sourceCollectionRef != destinationCollectionRef {
			if err := b.moveCalendarGroupGrants(sourceCollection, destinationCollection, sourceRef, destinationPath, tx); err != nil {
				return false, err
			}
		}
	} else if exists {
		if err := b.calendarUpdate(calendarObjectTable, destinationPath, destination, attrs, tx); err != nil {
			return false, err
		}
	} else if err := b.calendarCreateEvent(destinationPath, destinationCollection, attrs, tx); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return !exists, nil
}

func (b *DaptinDAVBackend) calendarEventAllowed(ref daptinid.DaptinReferenceId, tx *sqlx.Tx, method string) bool {
	crud := b.cruds[calendarObjectTable]
	table := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	row := GetObjectPermissionByReferenceIdWithTransaction(calendarObjectTable, ref, tx)
	user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId
	if method == http.MethodDelete {
		return table.CanDelete(user, groups, admin) && row.CanDelete(user, groups, admin)
	}
	return table.CanUpdate(user, groups, admin) && row.CanUpdate(user, groups, admin)
}

func (b *DaptinDAVBackend) moveCalendarGroupGrants(source, destination map[string]interface{}, eventRef daptinid.DaptinReferenceId, requestPath string, tx *sqlx.Tx) error {
	sourceGrants, err := b.calendarCollectionGroupGrants(source, tx)
	if err != nil {
		return err
	}
	destinationGrants, err := b.calendarCollectionGroupGrants(destination, tx)
	if err != nil {
		return err
	}
	share := calendarShareAction{cruds: b.cruds}
	for groupRef := range sourceGrants {
		if _, exists := destinationGrants[groupRef]; !exists {
			if err := share.setLink(calendarObjectTable, eventRef, groupRef, 0, requestPath, b.sessionUser, tx); err != nil {
				return err
			}
		}
	}
	for groupRef, grant := range destinationGrants {
		if err := share.setLink(calendarObjectTable, eventRef, groupRef, grant, requestPath, b.sessionUser, tx); err != nil {
			return err
		}
	}
	return nil
}

func (b *DaptinDAVBackend) calendarCollectionGroupGrants(collection map[string]interface{}, tx *sqlx.Tx) (map[daptinid.DaptinReferenceId]auth.AuthPermission, error) {
	return b.davCollectionGroupGrants(calendarCollectionTable, collection, tx)
}

func (b *DaptinDAVBackend) davCollectionGroupGrants(table string, collection map[string]interface{}, tx *sqlx.Tx) (map[daptinid.DaptinReferenceId]auth.AuthPermission, error) {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	collectionID, err := GetReferenceIdToIdWithTransaction(table, collectionRef, tx)
	if err != nil {
		return nil, err
	}
	joinTable := table + "_" + table + "_id_has_usergroup_usergroup_id"
	links, err := GetObjectByWhereClauseWithTransaction(joinTable, tx, goqu.Ex{table + "_id": collectionID})
	if err != nil {
		return nil, err
	}
	grants := make(map[daptinid.DaptinReferenceId]auth.AuthPermission, len(links))
	for _, link := range links {
		groupID, err := ResourceRowInt64(link["usergroup_id"])
		if err != nil {
			return nil, err
		}
		groupRef, err := GetIdToReferenceIdWithTransaction("usergroup", groupID, tx)
		if err != nil {
			return nil, err
		}
		bits, err := ResourceRowInt64(link["permission"])
		if err != nil || bits < 0 {
			return nil, fmt.Errorf("invalid collection group grant: %v", link["permission"])
		}
		grant := auth.AuthPermission(bits) & (auth.GroupPeek | auth.GroupRead | auth.GroupCreate | auth.GroupUpdate | auth.GroupDelete | auth.GroupRefer)
		if grant != 0 {
			grants[groupRef] = grant
		}
	}
	return grants, nil
}
