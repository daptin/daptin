package resource

import (
	"context"
	"errors"
	"net/http"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/caldav"
)

// A sync token records the protocol revision and effective access without
// walking or storing every object in the collection.
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
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	head, err := b.davLogHead(collectionRef, tx)
	if err != nil {
		return "", err
	}
	access, err := b.davSyncAccess(calendarCollectionTable, calendarObjectTable, collectionRef, tx)
	if err != nil {
		return "", err
	}
	token, err := b.saveDAVSyncState(collectionRef, davSyncCheckpoint{Phase: "steady", Cursor: head, Access: access}, tx)
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
	access, err := b.davSyncAccess(calendarCollectionTable, calendarObjectTable, collectionRef, tx)
	if err != nil {
		return nil, err
	}
	previous := davSyncCheckpoint{Phase: "initial", Cursor: head, Base: head, Access: access}
	if previousToken != "" {
		previous, err = b.loadDAVSyncState(collectionRef, previousToken, tx)
		if errors.Is(err, errInvalidDAVSyncToken) {
			return nil, caldav.ErrInvalidSyncToken
		}
		if err != nil {
			return nil, err
		}
		if previous.Access != access || previous.Cursor > head {
			return nil, caldav.ErrInvalidSyncToken
		}
	}
	visible, paths, next, more, err := b.davSyncPage(calendarObjectTable, calendarCollectionTable, "collection_id",
		requestPath, collectionRef, previous, head, limit, tx)
	if err != nil {
		return nil, err
	}
	result := &caldav.CalendarSyncResult{More: more, Changes: make([]caldav.CalendarSyncChange, 0, len(paths))}
	for _, member := range paths {
		if row, present := visible[member]; present {
			object, err := b.calendarObject(row, collection)
			if err != nil {
				return nil, err
			}
			result.Changes = append(result.Changes, caldav.CalendarSyncChange{Path: member, Object: &object})
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
