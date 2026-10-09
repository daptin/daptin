package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	encodingjson "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/caldav"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

const (
	maxDAVSyncObjects = 10000
	davSyncLifetime   = 30 * 24 * time.Hour
)

// The checkpoint is protocol history, not calendar membership authority.
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
	state, _, err := b.calendarSyncState(requestPath, collection, tx)
	if err != nil {
		return "", err
	}
	token, err := b.saveCalendarSyncState(collection, state, tx)
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
	previous := make(map[string]string)
	if previousToken != "" {
		previous, err = b.loadCalendarSyncState(collection, previousToken, tx)
		if err != nil {
			return nil, err
		}
	}
	current, objects, err := b.calendarSyncState(requestPath, collection, tx)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(previous)+len(current))
	for member := range current {
		if previous[member] != current[member] {
			paths = append(paths, member)
		}
	}
	for member := range previous {
		if _, stillPresent := current[member]; !stillPresent {
			paths = append(paths, member)
		}
	}
	sort.Strings(paths)
	more := len(paths) > limit
	if more {
		paths = paths[:limit]
	}
	next := make(map[string]string, len(previous)+len(paths))
	for member, etag := range previous {
		next[member] = etag
	}
	result := &caldav.CalendarSyncResult{More: more, Changes: make([]caldav.CalendarSyncChange, 0, len(paths))}
	for _, member := range paths {
		if object, present := objects[member]; present {
			copy := object
			result.Changes = append(result.Changes, caldav.CalendarSyncChange{Path: member, Object: &copy})
			next[member] = current[member]
		} else {
			result.Changes = append(result.Changes, caldav.CalendarSyncChange{Path: member})
			delete(next, member)
		}
	}
	result.Token, err = b.saveCalendarSyncState(collection, next, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

func (b *DaptinDAVBackend) calendarSyncState(requestPath string, collection map[string]interface{}, tx *sqlx.Tx) (map[string]string, map[string]caldav.CalendarObject, error) {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return nil, nil, err
	}
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction(calendarObjectTable, tx,
		maxDAVSyncObjects+1, goqu.Ex{"collection_id": collectionID})
	if err != nil {
		return nil, nil, err
	}
	if len(refs) > maxDAVSyncObjects {
		return nil, nil, webdav.NewHTTPError(http.StatusInsufficientStorage, errors.New("calendar has too many objects to sync"))
	}
	rows, err := b.calendarRowsWithTransaction(calendarObjectTable, requestPath, tx,
		Query{ColumnName: "collection_id", Operator: "=", Value: collectionRef.String()})
	if err != nil {
		return nil, nil, err
	}
	state := make(map[string]string, len(rows))
	objects := make(map[string]caldav.CalendarObject, len(rows))
	for _, row := range rows {
		object, err := b.calendarObject(row, collection)
		if err != nil {
			return nil, nil, err
		}
		state[object.Path] = object.ETag
		objects[object.Path] = object
	}
	return state, objects, nil
}

func (b *DaptinDAVBackend) loadCalendarSyncState(collection map[string]interface{}, token string, tx *sqlx.Tx) (map[string]string, error) {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	refs, err := GetReferenceIdByWhereClauseWithTransaction("cal_sync", tx, goqu.Ex{"token": token})
	if err != nil {
		return nil, err
	}
	for _, ref := range refs {
		row, _, err := b.cruds["cal_sync"].GetSingleRowByReferenceIdWithTransaction("cal_sync", ref, nil, tx)
		if err != nil {
			return nil, err
		}
		expires, err := ResourceRowInt64(row["expires_at"])
		if err != nil {
			return nil, err
		}
		if daptinid.InterfaceToDIR(row["collection_reference"]) != collectionRef ||
			daptinid.InterfaceToDIR(row["user_account_id"]) != b.sessionUser.UserReferenceId ||
			expires <= time.Now().Unix() {
			continue
		}
		var state map[string]string
		if err := encodingjson.Unmarshal([]byte(StringOrEmpty(row["state"])), &state); err != nil {
			return nil, err
		}
		return state, nil
	}
	return nil, caldav.ErrInvalidSyncToken
}

func (b *DaptinDAVBackend) saveCalendarSyncState(collection map[string]interface{}, state map[string]string, tx *sqlx.Tx) (string, error) {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	if err := b.pruneCalendarSyncState(collectionRef, tx); err != nil {
		return "", err
	}
	encoded, err := encodingjson.Marshal(state)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(collectionRef.String() + "\x00" + b.sessionUser.UserReferenceId.String() + "\x00" + string(encoded)))
	token := "urn:daptin:cal-sync:" + hex.EncodeToString(digest[:])
	refs, err := GetReferenceIdByWhereClauseWithTransaction("cal_sync", tx, goqu.Ex{"token": token})
	if err != nil {
		return "", err
	}
	expires := time.Now().Add(davSyncLifetime).Unix()
	if len(refs) != 0 {
		model := api2go.NewApi2GoModelWithData("cal_sync", nil, 0, nil, map[string]interface{}{
			"reference_id": refs[0].String(), "expires_at": expires,
		})
		if _, err := b.cruds["cal_sync"].updateAfterAuthorizationWithTransaction(model,
			b.request(http.MethodPatch, "/api/cal_sync/"+refs[0].String()), tx); err != nil {
			return "", davResourceError(err)
		}
		return token, nil
	}
	model := api2go.NewApi2GoModelWithData("cal_sync", nil, int64(b.cruds["cal_sync"].TableInfo().DefaultPermission), nil,
		map[string]interface{}{"token": token, "state": string(encoded), "expires_at": expires, "collection_reference": collectionRef.String()})
	if _, err := b.cruds["cal_sync"].createWithoutFilterAfterAuthorization(model,
		b.request(http.MethodPost, "/api/cal_sync"), tx); err != nil {
		return "", davResourceError(err)
	}
	return token, nil
}

func (b *DaptinDAVBackend) pruneCalendarSyncState(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("cal_sync", tx, 100,
		goqu.Ex{"collection_reference": collectionRef.String(), "expires_at": goqu.Op{"lt": time.Now().Unix()}})
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := b.cruds["cal_sync"].deleteAfterAuthorizationWithTransaction(ref,
			b.request(http.MethodDelete, fmt.Sprintf("/api/cal_sync/%s", ref)), tx); err != nil {
			return davResourceError(err)
		}
	}
	return nil
}

func (b *DaptinDAVBackend) deleteCalendarSyncState(collection map[string]interface{}, tx *sqlx.Tx) error {
	collectionRef := daptinid.InterfaceToDIR(collection["reference_id"])
	for {
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("cal_sync", tx, 100, goqu.Ex{"collection_reference": collectionRef.String()})
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			return nil
		}
		for _, ref := range refs {
			if _, err := b.cruds["cal_sync"].deleteAfterAuthorizationWithTransaction(ref,
				b.request(http.MethodDelete, fmt.Sprintf("/api/cal_sync/%s", ref)), tx); err != nil {
				return davResourceError(err)
			}
		}
	}
}
