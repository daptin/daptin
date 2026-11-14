package resource

import (
	"bytes"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/actionresponse"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

const calendarShareListPageSize = 100

type calendarListSharesAction struct {
	share *calendarShareAction
}

func NewCalendarListSharesAction(cruds map[string]*DbResource) actionresponse.ActionPerformerInterface {
	return &calendarListSharesAction{share: &calendarShareAction{cruds: cruds}}
}

func (*calendarListSharesAction) Name() string { return "calendar.list_shares" }

func (a *calendarListSharesAction) decodeCursor(value interface{}, caller, collection daptinid.DaptinReferenceId) (daptinid.DaptinReferenceId, error) {
	encoded, ok := value.(string)
	if !ok || len(encoded) != base64.RawURLEncoding.EncodedLen(12+48+16) {
		return daptinid.NullReferenceId, errors.New("invalid share cursor")
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(data) != 12+48+16 {
		return daptinid.NullReferenceId, errors.New("invalid share cursor")
	}
	plain, err := AESGCMDecrypt(a.share.cruds[calendarCollectionTable].EncryptionSecret, data[:12], data[12:])
	if err != nil || len(plain) != 48 || !bytes.Equal(plain[:16], caller[:]) || !bytes.Equal(plain[16:32], collection[:]) {
		return daptinid.NullReferenceId, errors.New("invalid share cursor")
	}
	var last daptinid.DaptinReferenceId
	copy(last[:], plain[32:])
	return last, nil
}

func (a *calendarListSharesAction) encodeCursor(last, caller, collection daptinid.DaptinReferenceId) (string, error) {
	plain := make([]byte, 0, 48)
	plain = append(plain, caller[:]...)
	plain = append(plain, collection[:]...)
	plain = append(plain, last[:]...)
	ciphertext, nonce, err := AESGCMEncrypt(a.share.cruds[calendarCollectionTable].EncryptionSecret, plain)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

func (a *calendarListSharesAction) DoAction(_ actionresponse.Outcome, fields map[string]interface{}, tx *sqlx.Tx) (api2go.Responder, []actionresponse.ActionResponse, []error) {
	if tx == nil {
		return nil, nil, []error{errors.New("calendar shares require an action transaction")}
	}
	caller, ok := fields["sessionUser"].(*auth.SessionUser)
	if !ok || caller == nil || caller.UserReferenceId == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("authentication required"), "authentication required", http.StatusUnauthorized)}
	}
	collectionRef := daptinid.InterfaceToDIR(fields["collection_id"])
	if collectionRef == daptinid.NullReferenceId {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("invalid reference ID"), "invalid reference ID", http.StatusBadRequest)}
	}
	collectionID, err := GetReferenceIdToIdWithTransaction(calendarCollectionTable, collectionRef, tx)
	if err != nil {
		return nil, nil, []error{api2go.NewHTTPError(err, "collection not found", http.StatusNotFound)}
	}
	active, canManage := a.share.managementAccess(collectionRef, caller, tx)
	if !canManage {
		return nil, nil, []error{api2go.NewHTTPError(errors.New("calendar shares denied"), "calendar shares denied", http.StatusForbidden)}
	}
	filters := []goqu.Ex{{"collection_id": collectionID}}
	if value, present := fields["cursor"]; present && value != nil && value != "" {
		last, err := a.decodeCursor(value, active.UserReferenceId, collectionRef)
		if err != nil {
			return nil, nil, []error{api2go.NewHTTPError(err, "invalid share cursor", http.StatusBadRequest)}
		}
		filters = append(filters, goqu.Ex{"reference_id": goqu.Op{"gt": last[:]}})
	}
	links, err := GetLimitedOrderedRowsWithTransaction("collection_collection_id_has_usergroup_usergroup_id",
		[]string{"reference_id", "usergroup_id", "permission"}, "reference_id", tx, calendarShareListPageSize+1, filters...)
	if err != nil {
		return nil, nil, []error{err}
	}
	more := len(links) > calendarShareListPageSize
	if more {
		links = links[:calendarShareListPageSize]
	}
	protectedID, _, err := a.share.protectedOwnerShareGroup(collectionRef, collectionID, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	shares, err := a.shares(collectionRef, protectedID, links, tx)
	if err != nil {
		return nil, nil, []error{err}
	}
	nextCursor := ""
	if more {
		nextCursor, err = a.encodeCursor(daptinid.InterfaceToDIR(links[len(links)-1]["reference_id"]), active.UserReferenceId, collectionRef)
		if err != nil {
			return nil, nil, []error{err}
		}
	}
	return nil, []actionresponse.ActionResponse{NewActionResponse("calendar.list_shares", map[string]interface{}{
		"collection_id": collectionRef.String(), "shares": shares, "next_cursor": nextCursor,
	})}, nil
}

func (a *calendarListSharesAction) shares(collectionRef daptinid.DaptinReferenceId, protectedID int64, links []map[string]interface{}, tx *sqlx.Tx) ([]map[string]interface{}, error) {
	visible := make([]map[string]interface{}, 0)
	if len(links) == 0 {
		return visible, nil
	}
	groupIDs := make([]int64, 0, len(links))
	for _, link := range links {
		id, err := ResourceRowInt64(link["usergroup_id"])
		if err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, id)
	}
	groups, err := GetLimitedOrderedRowsWithTransaction("usergroup", []string{"id", "name"}, "id", tx,
		uint(len(groupIDs)), goqu.Ex{"id": goqu.Op{"in": groupIDs}})
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(groups))
	directGroupIDs := make([]int64, 0, len(groups))
	prefix := "calshare:" + strings.ReplaceAll(collectionRef.String(), "-", "") + ":"
	for _, group := range groups {
		id, err := ResourceRowInt64(group["id"])
		if err != nil {
			return nil, err
		}
		name := StringOrEmpty(group["name"])
		if strings.HasPrefix(name, prefix) {
			names[id] = name
			directGroupIDs = append(directGroupIDs, id)
		}
	}
	if len(directGroupIDs) == 0 {
		return visible, nil
	}
	members, err := GetLimitedOrderedRowsWithTransaction("user_account_user_account_id_has_usergroup_usergroup_id",
		[]string{"usergroup_id", "user_account_id"}, "usergroup_id", tx, uint(2*len(directGroupIDs)+1),
		goqu.Ex{"usergroup_id": goqu.Op{"in": directGroupIDs}})
	if err != nil {
		return nil, err
	}
	if len(members) > 2*len(directGroupIDs) {
		return nil, errors.New("calendar share membership limit exceeded")
	}
	membership := make(map[int64][]int64, len(directGroupIDs))
	accountIDs := make([]int64, 0, len(members))
	for _, member := range members {
		groupID, err := ResourceRowInt64(member["usergroup_id"])
		if err != nil {
			return nil, err
		}
		accountID, err := ResourceRowInt64(member["user_account_id"])
		if err != nil {
			return nil, err
		}
		membership[groupID] = append(membership[groupID], accountID)
		if len(membership[groupID]) == 1 {
			accountIDs = append(accountIDs, accountID)
		}
	}
	if len(accountIDs) == 0 {
		return visible, nil
	}
	accounts, err := GetLimitedOrderedRowsWithTransaction(USER_ACCOUNT_TABLE_NAME,
		[]string{"id", "reference_id", "name", "email"}, "id", tx, uint(len(accountIDs)),
		goqu.Ex{"id": goqu.Op{"in": accountIDs}})
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]map[string]interface{}, len(accounts))
	for _, account := range accounts {
		id, err := ResourceRowInt64(account["id"])
		if err != nil {
			return nil, err
		}
		byID[id] = account
	}
	for _, link := range links {
		groupID, err := ResourceRowInt64(link["usergroup_id"])
		if err != nil {
			return nil, err
		}
		if groupID == protectedID {
			continue
		}
		members := membership[groupID]
		if len(members) != 1 {
			continue
		}
		account := byID[members[0]]
		accountRef := daptinid.InterfaceToDIR(account["reference_id"])
		if davAccountShareTarget("calshare", collectionRef, names[groupID], []daptinid.DaptinReferenceId{accountRef}) == daptinid.NullReferenceId {
			continue
		}
		grant, err := davACLLinkGrant(link["permission"])
		if err != nil {
			return nil, err
		}
		if grant == 0 {
			continue
		}
		item := map[string]interface{}{
			"user_account_id": accountRef.String(), "permission": int64(grant),
			"can_free_busy": grant&(auth.GroupPeek|auth.GroupRead) != 0,
			"can_read":      grant&auth.GroupRead != 0,
			"can_edit":      grant&(auth.GroupRead|auth.GroupUpdate) == auth.GroupRead|auth.GroupUpdate,
		}
		// Management access and the persisted one-account link scope these labels.
		item["name"] = StringOrEmpty(account["name"])
		item["email"] = StringOrEmpty(account["email"])
		visible = append(visible, item)
	}
	return visible, nil
}
