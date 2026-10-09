package resource

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-ical"
)

const maxDAVCollectionTransferObjects = 10000

func (b *DaptinDAVBackend) CopyCalendarCollection(_ context.Context, sourcePath, destinationPath string, recursive, overwrite bool) (bool, error) {
	return b.transferCalendarCollection(sourcePath, destinationPath, recursive, overwrite, false)
}

func (b *DaptinDAVBackend) MoveCalendarCollection(_ context.Context, sourcePath, destinationPath string, overwrite bool) (bool, error) {
	return b.transferCalendarCollection(sourcePath, destinationPath, true, overwrite, true)
}

func (b *DaptinDAVBackend) transferCalendarCollection(sourcePath, destinationPath string, recursive, overwrite, move bool) (bool, error) {
	sourceOwner, sourceName, err := b.calendarPath(sourcePath, false)
	if err != nil {
		return false, err
	}
	destinationOwner, destinationName, err := b.calendarPath(destinationPath, false)
	if err != nil {
		return false, err
	}
	if path.Clean(sourcePath) == path.Clean(destinationPath) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar source and destination are the same"))
	}
	if destinationOwner != b.sessionUser.UserReferenceId || move && sourceOwner != destinationOwner {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar destination must be in the owner's home"))
	}
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	source, err := b.calendarCollection(sourceOwner, sourceName, sourcePath, tx)
	if err != nil {
		return false, err
	}
	destination, err := b.calendarCollection(destinationOwner, destinationName, destinationPath, tx)
	exists := err == nil
	if err != nil && !errors.Is(err, errDAVNotFound) {
		return false, err
	}
	if exists && !overwrite {
		return false, webdav.NewHTTPError(http.StatusPreconditionFailed, errors.New("calendar destination exists"))
	}
	sourceRef := daptinid.InterfaceToDIR(source["reference_id"])
	if exists && sourceRef == daptinid.InterfaceToDIR(destination["reference_id"]) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar source and destination are the same"))
	}
	first, second := source, destination
	if exists && sourceRef.String() > daptinid.InterfaceToDIR(destination["reference_id"]).String() {
		first, second = destination, source
	}
	if err := b.lockCalendarCollection(first, tx); err != nil {
		return false, err
	}
	if exists {
		if err := b.lockCalendarCollection(second, tx); err != nil {
			return false, err
		}
	}
	collectionCRUD := b.cruds[calendarCollectionTable]
	collectionTable := collectionCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	collectionRow := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, sourceRef, tx)
	user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, collectionCRUD.AdministratorGroupId
	if !collectionTable.CanRead(user, groups, admin) || !collectionRow.CanRead(user, groups, admin) ||
		move && (!collectionTable.CanUpdate(user, groups, admin) || !collectionRow.CanUpdate(user, groups, admin)) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar transfer denied"))
	}
	eventCRUD := b.cruds[calendarObjectTable]
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	if recursive && (!eventTable.CanRead(user, groups, admin) || !move && !eventTable.CanCreate(user, groups, admin) ||
		move && !eventTable.CanUpdate(user, groups, admin)) {
		return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar object transfer denied"))
	}
	var objects []map[string]interface{}
	if recursive {
		collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, sourceRef, tx)
		if err != nil {
			return false, err
		}
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx,
			maxDAVCollectionTransferObjects+1, goqu.Ex{"collection_id": collectionID})
		if err != nil {
			return false, err
		}
		if len(refs) > maxDAVCollectionTransferObjects {
			return false, webdav.NewHTTPError(http.StatusInsufficientStorage, errors.New("calendar has too many objects to transfer"))
		}
		objects, err = b.calendarRowsWithTransaction(calendarObjectTable, sourcePath, tx,
			Query{ColumnName: "collection_id", Operator: "=", Value: sourceRef.String()})
		if err != nil {
			return false, err
		}
		if len(objects) != len(refs) {
			return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar contains an unreadable object"))
		}
		if move {
			for _, object := range objects {
				if !b.calendarEventAllowed(daptinid.InterfaceToDIR(object["reference_id"]), tx, http.MethodPatch) {
					return false, webdav.NewHTTPError(http.StatusForbidden, errors.New("calendar contains an uneditable object"))
				}
			}
		}
	}
	if exists {
		if err := b.deleteCalendarCollection(destinationPath, destination, tx); err != nil {
			return false, err
		}
	}
	if move {
		if err := b.calendarUpdate(calendarCollectionTable, sourcePath, source, map[string]interface{}{"name": destinationName}, tx); err != nil {
			return false, err
		}
		for _, object := range objects {
			oldPath := fmt.Sprint(object["rpath"])
			newPath := path.Join(destinationPath, path.Base(oldPath))
			content, err := b.contentBytes(calendarObjectTable, object)
			if err != nil {
				return false, err
			}
			if err := b.calendarUpdate(calendarObjectTable, oldPath, object, map[string]interface{}{
				"rpath": newPath, "content": b.contentValue(calendarObjectTable, newPath, ical.MIMEType, content),
			}, tx); err != nil {
				return false, err
			}
		}
	} else {
		attrs := map[string]interface{}{"name": destinationName, "description": StringOrEmpty(source["description"])}
		if displayName := StringOrEmpty(source["display_name"]); displayName != "" {
			attrs["display_name"] = displayName
		}
		if err := b.calendarCreate(calendarCollectionTable, destinationPath, attrs, tx); err != nil {
			return false, err
		}
		created, err := b.calendarCollection(destinationOwner, destinationName, destinationPath, tx)
		if err != nil {
			return false, err
		}
		for _, object := range objects {
			newPath := path.Join(destinationPath, path.Base(fmt.Sprint(object["rpath"])))
			content, err := b.contentBytes(calendarObjectTable, object)
			if err != nil {
				return false, err
			}
			calendar, err := ical.NewDecoder(bytes.NewReader(content)).Decode()
			if err != nil {
				return false, err
			}
			uid, organizer := itipCalendarIdentity(calendar)
			for _, event := range itipComponents(calendar) {
				for i := range event.Props["ATTENDEE"] {
					event.Props["ATTENDEE"][i].Params.Del("SCHEDULE-STATUS")
				}
				for i := range event.Props["ORGANIZER"] {
					event.Props["ORGANIZER"][i].Params.Del("SCHEDULE-STATUS")
				}
			}
			var encoded bytes.Buffer
			if err := ical.NewEncoder(&encoded).Encode(calendar); err != nil {
				return false, err
			}
			if err := b.calendarCreateEvent(newPath, created, map[string]interface{}{
				"rpath": newPath, "collection_id": fmt.Sprint(created["reference_id"]),
				"content": b.contentValue(calendarObjectTable, newPath, ical.MIMEType, encoded.Bytes()),
				"uid":     uid, "organizer_address": organizer,
			}, tx); err != nil {
				return false, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return !exists, nil
}
