package resource

import (
	"context"
	"errors"
	"net/http"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/caldav"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// The checkpoint is protocol history, not collection membership authority.
// Each report obtains current membership through permission-filtered resources.
func (b *DaptinDAVBackend) CalendarSyncToken(_ context.Context, requestPath string) (string, error) {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return "", err
	}
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return "", err
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return "", err
	}
	state, err := b.calendarSyncState(requestPath, collection, tx)
	if err != nil {
		return "", err
	}
	head, err := b.davLogHead(daptinid.InterfaceToDIR(collection["reference_id"]), tx)
	if err != nil {
		return "", err
	}
	token, err := b.saveDAVSyncState(daptinid.InterfaceToDIR(collection["reference_id"]), davSyncCheckpoint{Members: state, Cursor: head}, tx)
	if err != nil {
		return "", err
	}
	return token, tx.Commit()
}

func (b *DaptinDAVBackend) SyncCalendarObjects(_ context.Context, requestPath, previousToken string, limit int) (*caldav.CalendarSyncResult, error) {
	owner, name, err := b.calendarPath(requestPath, false)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > 1000 {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, errors.New("invalid calendar sync limit"))
	}
	tx, err := b.cruds[calendarCollectionTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	collection, err := b.calendarCollection(owner, name, requestPath, tx)
	if err != nil {
		return nil, err
	}
	if err := b.lockCalendarCollection(collection, tx); err != nil {
		return nil, err
	}
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	head, err := b.davLogHead(collectionRef, tx)
	if err != nil {
		return nil, err
	}
	previous := davSyncCheckpoint{Members: make(map[string]string), Cursor: head}
	if previousToken != "" {
		previous, err = b.loadDAVSyncState(collectionRef, previousToken, tx)
		if err != nil {
			if errors.Is(err, errInvalidDAVSyncToken) {
				return nil, caldav.ErrInvalidSyncToken
			}
			return nil, err
		}
	}
	if previous.Members == nil {
		previous.Members = make(map[string]string)
	}
	current, err := b.calendarSyncState(requestPath, collection, tx)
	if err != nil {
		return nil, err
	}
	logged, err := b.davLogChanges(collectionRef, previous.Cursor, head, tx)
	if err != nil {
		if errors.Is(err, errInvalidDAVSyncToken) {
			return nil, caldav.ErrInvalidSyncToken
		}
		return nil, err
	}
	eventCRUD := b.cruds[calendarObjectTable]
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	canReadEventTable := eventTable.CanRead(b.sessionUser.UserReferenceId, b.sessionUser.Groups, eventCRUD.AdministratorGroupId)
	paths, more := davSyncPaths(previous, current, logged, limit, func(change davLogChange) bool {
		return canReadEventTable && change.removed && change.readGrant.CanRead(b.sessionUser.UserReferenceId,
			b.sessionUser.Groups, eventCRUD.AdministratorGroupId)
	})
	next := davNextCheckpoint(previous, current, logged, paths, head, more)
	result := &caldav.CalendarSyncResult{More: more, Changes: make([]caldav.CalendarSyncChange, 0, len(paths))}
	objects, err := b.calendarSyncObjects(collection, paths, current, tx)
	if err != nil {
		return nil, err
	}
	for _, member := range paths {
		if object, present := objects[member]; present {
			copy := object
			result.Changes = append(result.Changes, caldav.CalendarSyncChange{Path: member, Object: &copy})
		} else {
			result.Changes = append(result.Changes, caldav.CalendarSyncChange{Path: member})
		}
	}
	result.Token, err = b.saveDAVSyncState(collectionRef, next, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (b *DaptinDAVBackend) calendarSyncState(requestPath string, collection map[string]interface{}, tx *sqlx.Tx) (map[string]string, error) {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	state := make(map[string]string)
	err := b.eachDAVRowWithTransaction(calendarObjectTable, requestPath, tx, func(row map[string]interface{}) error {
		data, err := b.contentBytes(calendarObjectTable, row)
		if err != nil {
			return err
		}
		state[StringOrEmpty(row["rpath"])] = GetMD5Hash(data)
		return nil
	}, Query{ColumnName: "collection_id", Operator: "=", Value: collectionRef.String()})
	return state, err
}

func (b *DaptinDAVBackend) calendarSyncObjects(collection map[string]interface{}, paths []string, current map[string]string, tx *sqlx.Tx) (map[string]caldav.CalendarObject, error) {
	wanted := make([]string, 0, len(paths))
	for _, member := range paths {
		if _, present := current[member]; present {
			wanted = append(wanted, member)
		}
	}
	objects := make(map[string]caldav.CalendarObject, len(wanted))
	if len(wanted) == 0 {
		return objects, nil
	}
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, daptinid.InterfaceToDIR(collection["reference_id"]), tx)
	if err != nil {
		return nil, err
	}
	rows, err := b.rowsWithTransaction(calendarObjectTable, tx, goqu.Ex{"collection_id": collectionID, "rpath": goqu.Op{"in": wanted}})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		object, err := b.calendarObject(row, collection)
		if err != nil {
			return nil, err
		}
		objects[object.Path] = object
	}
	return objects, nil
}
