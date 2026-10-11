package resource

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
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

const calendarDiscoveryPageSize = 100

const calendarDiscoveryNonceSize = 12

type calendarDiscoverAction struct {
	cruds map[string]*DbResource
}

func NewCalendarDiscoverAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &calendarDiscoverAction{cruds: cruds}
}

func (*calendarDiscoverAction) Name() string { return "calendar.discover" }

func calendarDiscoveryAccess(user daptinid.DaptinReferenceId, groups auth.GroupPermissionList,
	collectionTable, collection, eventTable permission.PermissionInstance,
	collectionAdmin, eventAdmin daptinid.DaptinReferenceId) (bool, bool) {
	rowRead := collection.CanRead(user, groups, collectionAdmin)
	collectionPeek := collectionTable.CanPeek(user, groups, collectionAdmin)
	eventPeek := eventTable.CanPeek(user, groups, eventAdmin)
	canRead := collectionPeek && rowRead && eventPeek
	canFreeBusy := (collectionPeek || collectionTable.CanRead(user, groups, collectionAdmin)) &&
		(collection.CanPeek(user, groups, collectionAdmin) || rowRead) &&
		(eventPeek || eventTable.CanRead(user, groups, eventAdmin))
	return canRead, canFreeBusy
}

func (a *calendarDiscoverAction) decodeCursor(value interface{}, user daptinid.DaptinReferenceId) (daptinid.DaptinReferenceId, error) {
	encoded, ok := value.(string)
	if !ok || len(encoded) != base64.RawURLEncoding.EncodedLen(calendarDiscoveryNonceSize+32+16) {
		return daptinid.NullReferenceId, errors.New("invalid calendar cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(data) != calendarDiscoveryNonceSize+32+16 {
		return daptinid.NullReferenceId, errors.New("invalid calendar cursor")
	}
	plain, err := AESGCMDecrypt(a.cruds[calendarCollectionTable].EncryptionSecret,
		data[:calendarDiscoveryNonceSize], data[calendarDiscoveryNonceSize:])
	if err != nil || len(plain) != 32 || !bytes.Equal(plain[:16], user[:]) {
		return daptinid.NullReferenceId, errors.New("invalid calendar cursor")
	}
	var cursor daptinid.DaptinReferenceId
	copy(cursor[:], plain[16:])
	return cursor, nil
}

func (a *calendarDiscoverAction) encodeCursor(ref, user daptinid.DaptinReferenceId) (string, error) {
	plain := make([]byte, 0, 32)
	plain = append(plain, user[:]...)
	plain = append(plain, ref[:]...)
	ciphertext, nonce, err := AESGCMEncrypt(a.cruds[calendarCollectionTable].EncryptionSecret, plain)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (a *calendarDiscoverAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar discovery requires an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || caller.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}

	filters := make([]goqu.Ex, 0, 1)
	if value, present := fields["cursor"]; present && value != nil && value != "" {
		cursor, err := a.decodeCursor(value, caller.UserReferenceId)
		if err != nil {
			return nil, nil, []error{api2go.NewHTTPError(err, "invalid calendar cursor", http.StatusBadRequest)}
		}
		filters = append(filters, goqu.Ex{"reference_id": goqu.Op{"gt": cursor[:]}})
	}

	// Action outcomes receive temporary privileges. Discoverability uses only
	// the account's persisted membership and the ordinary table and row grants.
	groups := GetObjectGroupsByObjectIdWithTransaction(USER_ACCOUNT_TABLE_NAME, caller.UserId, tx)
	user := caller.UserReferenceId
	collectionCRUD := a.cruds[calendarCollectionTable]
	collectionTable := collectionCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarCollectionTable, tx)
	eventCRUD := a.cruds[calendarObjectTable]
	eventTable := eventCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", calendarObjectTable, tx)
	admin := collectionCRUD.AdministratorGroupId
	eventAdmin := eventCRUD.AdministratorGroupId

	rows, err := GetLimitedOrderedRowsWithTransaction(calendarCollectionTable,
		[]string{"reference_id", "user_account_id", "name", "display_name"}, "reference_id", tx,
		calendarDiscoveryPageSize+1, filters...)
	if err != nil {
		return nil, nil, []error{err}
	}
	more := len(rows) > calendarDiscoveryPageSize
	if more {
		rows = rows[:calendarDiscoveryPageSize]
	}
	type candidate struct {
		ref            daptinid.DaptinReferenceId
		ownerID        int64
		name, label    string
		read, freeBusy bool
	}
	pending := make([]candidate, 0)
	ownerIDs := make(map[int64]bool)
	for _, row := range rows {
		ref := daptinid.InterfaceToDIR(row["reference_id"])
		if ref == daptinid.NullReferenceId {
			return nil, nil, []error{errors.New("calendar discovery found an invalid collection reference")}
		}
		grant := GetObjectPermissionByReferenceIdWithTransaction(calendarCollectionTable, ref, tx)
		canRead, canFreeBusy := calendarDiscoveryAccess(user, groups, collectionTable, grant, eventTable, admin, eventAdmin)
		if !canRead && !canFreeBusy {
			continue
		}
		ownerID, err := ResourceRowInt64(row["user_account_id"])
		if err != nil {
			return nil, nil, []error{err}
		}
		name := StringOrEmpty(row["name"])
		label := name
		if canRead && StringOrEmpty(row["display_name"]) != "" {
			label = StringOrEmpty(row["display_name"])
		}
		ownerIDs[ownerID] = true
		pending = append(pending, candidate{ref, ownerID, name, label, canRead, canFreeBusy})
	}
	owners := make(map[int64]daptinid.DaptinReferenceId, len(ownerIDs))
	if len(ownerIDs) > 0 {
		ids := make([]int64, 0, len(ownerIDs))
		for id := range ownerIDs {
			ids = append(ids, id)
		}
		ownerRows, err := GetLimitedOrderedRowsWithTransaction(USER_ACCOUNT_TABLE_NAME,
			[]string{"id", "reference_id"}, "id", tx, uint(len(ids)), goqu.Ex{"id": goqu.Op{"in": ids}})
		if err != nil {
			return nil, nil, []error{err}
		}
		for _, row := range ownerRows {
			id, err := ResourceRowInt64(row["id"])
			if err != nil {
				return nil, nil, []error{err}
			}
			owners[id] = daptinid.InterfaceToDIR(row["reference_id"])
		}
	}
	visible := make([]map[string]interface{}, 0, len(pending))
	for _, item := range pending {
		owner := owners[item.ownerID]
		if owner == daptinid.NullReferenceId {
			return nil, nil, []error{errors.New("calendar discovery found an invalid owner reference")}
		}
		visible = append(visible, map[string]interface{}{
			"collection_id": item.ref.String(),
			"href":          fmt.Sprintf("/caldav/%s/calendars/%s/", owner, url.PathEscape(item.name)),
			"display_name":  item.label,
			"can_read":      item.read,
			"can_free_busy": item.freeBusy,
		})
	}

	nextCursor := ""
	if more {
		nextCursor, err = a.encodeCursor(daptinid.InterfaceToDIR(rows[len(rows)-1]["reference_id"]), user)
		if err != nil {
			return nil, nil, []error{err}
		}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.discover", map[string]interface{}{
		"collections": visible, "next_cursor": nextCursor,
	})}, nil
}
