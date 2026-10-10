package resource

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"path"
	"time"

	"github.com/artpar/api2go/v2"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

var errInvalidDAVSyncToken = errors.New("invalid DAV sync token")

type davLogChange struct {
	revision  int64
	path      string
	removed   bool
	readGrant permission.PermissionInstance
}

// The log records only protocol writes. Its rows and the event mutation share
// the caller's transaction, so a failed or rolled-back mutation leaves no
// visible history. The collection lock orders committed revisions.
func (b *DaptinDAVBackend) appendCalendarChange(collectionRef daptinid.DaptinReferenceId, requestPath string,
	removedGrant *permission.PermissionInstance, tx *sqlx.Tx) error {
	return b.appendDAVChange(calendarCollectionTable, collectionRef, requestPath, removedGrant, tx)
}

func (b *DaptinDAVBackend) appendDAVChange(collectionTable string, collectionRef daptinid.DaptinReferenceId, requestPath string,
	removedGrant *permission.PermissionInstance, tx *sqlx.Tx) error {
	if collectionRef == daptinid.NullReferenceId || requestPath == "" {
		return fmt.Errorf("DAV change has no collection or path")
	}
	if err := b.cruds[collectionTable].lockRowByWhereWithTransaction(tx,
		goqu.Ex{"reference_id": collectionRef[:]}); err != nil {
		return err
	}
	revision, err := b.advanceDAVClock(collectionRef, tx)
	if err != nil {
		return err
	}
	attrs := map[string]interface{}{
		"collection_reference": collectionRef.String(),
		"revision":             revision,
		"rpath":                path.Clean(requestPath),
		"removed":              int64(0),
		"expires_at":           time.Now().Add(davSyncLifetime).Unix(),
	}
	if removedGrant != nil {
		encoded, err := removedGrant.MarshalBinary()
		if err != nil {
			return err
		}
		attrs["removed"] = int64(1)
		attrs["read_grant"] = base64.StdEncoding.EncodeToString(encoded)
	}
	crud := b.cruds["dav_log"]
	model := api2go.NewApi2GoModelWithData("dav_log", nil, int64(crud.TableInfo().DefaultPermission), nil, attrs)
	_, err = crud.createWithoutFilterAfterAuthorization(model, b.request(http.MethodPost, "/api/dav_log"), tx)
	return davResourceError(err)
}

func (b *DaptinDAVBackend) davLogHead(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) (int64, error) {
	refs, err := GetReferenceIdByWhereClauseWithTransaction("dav_clock", tx,
		goqu.Ex{"collection_reference": collectionRef.String()})
	if err != nil || len(refs) == 0 {
		return 0, err
	}
	row, _, err := b.cruds["dav_clock"].GetSingleRowByReferenceIdWithTransaction("dav_clock", refs[0], nil, tx)
	if err != nil {
		return 0, err
	}
	return ResourceRowInt64(row["revision"])
}

func (b *DaptinDAVBackend) advanceDAVClock(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) (int64, error) {
	refs, err := GetReferenceIdByWhereClauseWithTransaction("dav_clock", tx,
		goqu.Ex{"collection_reference": collectionRef.String()})
	if err != nil {
		return 0, err
	}
	crud := b.cruds["dav_clock"]
	if len(refs) == 0 {
		model := api2go.NewApi2GoModelWithData("dav_clock", nil, int64(crud.TableInfo().DefaultPermission), nil,
			map[string]interface{}{"collection_reference": collectionRef.String(), "revision": int64(1)})
		_, err := crud.createWithoutFilterAfterAuthorization(model, b.request(http.MethodPost, "/api/dav_clock"), tx)
		return 1, davResourceError(err)
	}
	row, _, err := crud.GetSingleRowByReferenceIdWithTransaction("dav_clock", refs[0], nil, tx)
	if err != nil {
		return 0, err
	}
	previous, err := ResourceRowInt64(row["revision"])
	if err != nil {
		return 0, err
	}
	model := api2go.NewApi2GoModelWithData("dav_clock", nil, 0, nil,
		map[string]interface{}{"reference_id": refs[0].String(), "revision": previous + 1})
	_, err = crud.updateAfterAuthorizationWithTransaction(model,
		b.request(http.MethodPatch, "/api/dav_clock/"+refs[0].String()), tx)
	return previous + 1, davResourceError(err)
}

func (b *DaptinDAVBackend) davLogChanges(collectionRef daptinid.DaptinReferenceId, after, through int64,
	tx *sqlx.Tx) (map[string]davLogChange, error) {
	changes := make(map[string]davLogChange)
	if after >= through {
		return changes, nil
	}
	filters := []goqu.Ex{
		{"collection_reference": collectionRef.String()},
		{"revision": goqu.Op{"gt": after}},
		{"revision": goqu.Op{"lte": through}},
	}
	rows, _, err := b.cruds["dav_log"].GetRowsByWhereClauseWithTransaction("dav_log", nil, tx, filters...)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		revision, err := ResourceRowInt64(row["revision"])
		if err != nil {
			return nil, err
		}
		member := StringOrEmpty(row["rpath"])
		if current, exists := changes[member]; exists && current.revision > revision {
			continue
		}
		removed, err := ResourceRowInt64(row["removed"])
		if err != nil {
			return nil, err
		}
		change := davLogChange{revision: revision, path: member, removed: removed != 0}
		if change.removed {
			encoded, err := base64.StdEncoding.DecodeString(StringOrEmpty(row["read_grant"]))
			if err != nil || len(encoded) < 24 {
				return nil, fmt.Errorf("invalid DAV change permission")
			}
			if err := change.readGrant.UnmarshalBinary(encoded); err != nil {
				return nil, err
			}
		}
		changes[member] = change
	}
	return changes, nil
}

func (b *DaptinDAVBackend) pruneDAVLog(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("dav_log", tx, 100,
		goqu.Ex{"collection_reference": collectionRef.String(), "expires_at": goqu.Op{"lt": time.Now().Unix()}})
	if err != nil {
		return err
	}
	return b.deleteDAVLogRows(refs, tx)
}

func (b *DaptinDAVBackend) deleteDAVLogState(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	for {
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("dav_log", tx, 100,
			goqu.Ex{"collection_reference": collectionRef.String()})
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			return nil
		}
		if err := b.deleteDAVLogRows(refs, tx); err != nil {
			return err
		}
	}
}

func (b *DaptinDAVBackend) deleteDAVClock(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	refs, err := GetReferenceIdByWhereClauseWithTransaction("dav_clock", tx,
		goqu.Ex{"collection_reference": collectionRef.String()})
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := b.cruds["dav_clock"].deleteAfterAuthorizationWithTransaction(ref,
			b.request(http.MethodDelete, "/api/dav_clock/"+ref.String()), tx); err != nil {
			return davResourceError(err)
		}
	}
	return nil
}

func (b *DaptinDAVBackend) deleteDAVLogRows(refs []daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	for _, ref := range refs {
		if _, err := b.cruds["dav_log"].deleteAfterAuthorizationWithTransaction(ref,
			b.request(http.MethodDelete, "/api/dav_log/"+ref.String()), tx); err != nil {
			return davResourceError(err)
		}
	}
	return nil
}
