package resource

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	encodingjson "encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/permission"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

const (
	davSyncLifetime    = 30 * 24 * time.Hour
	davSyncSegmentSize = 32 * 1024
)

type davSyncCheckpoint struct {
	Phase   string           `json:"phase"`
	Path    string           `json:"path,omitempty"`
	Cursor  int64            `json:"cursor"`
	Base    int64            `json:"base,omitempty"`
	Access  string           `json:"access"`
	Pending []davSyncPending `json:"pending,omitempty"`
}

type davSyncPending struct {
	Path      string                        `json:"path"`
	Removed   bool                          `json:"removed,omitempty"`
	ReadGrant permission.PermissionInstance `json:"read_grant,omitempty"`
}

func (b *DaptinDAVBackend) davSyncAccess(collectionTable, objectTable string, collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) (string, error) {
	accountGroups := GetObjectGroupsByObjectIdWithTransaction(USER_ACCOUNT_TABLE_NAME, b.sessionUser.UserId, tx)
	collectionGrant := GetObjectPermissionByReferenceIdWithTransaction(collectionTable, collectionRef, tx)
	collectionTableGrant := b.cruds[collectionTable].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", collectionTable, tx)
	objectTableGrant := b.cruds[objectTable].GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", objectTable, tx)
	hash := sha256.New()
	writeGroups := func(groups auth.GroupPermissionList) {
		values := make([]string, 0, len(groups))
		for _, group := range groups {
			values = append(values, fmt.Sprintf("%s:%s:%s:%d", group.GroupReferenceId, group.ObjectReferenceId, group.RelationReferenceId, group.Permission))
		}
		sort.Strings(values)
		for _, value := range values {
			_, _ = hash.Write([]byte(value + "\n"))
		}
	}
	writeGrant := func(grant permission.PermissionInstance) {
		_, _ = fmt.Fprintf(hash, "%s:%d\n", grant.UserId, grant.Permission)
		writeGroups(grant.UserGroupId)
	}
	writeGroups(accountGroups)
	writeGrant(collectionGrant)
	writeGrant(collectionTableGrant)
	writeGrant(objectTableGrant)
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// davSyncVisibleRows enters the ordinary permission-filtered resource list for
// a bounded set of candidate references or paths.
func (b *DaptinDAVBackend) davSyncVisibleRows(table, requestPath, collectionColumn string, collectionRef daptinid.DaptinReferenceId,
	filter Query, limit int, tx *sqlx.Tx) (map[string]map[string]interface{}, error) {
	query, err := encodingjson.Marshal([]Query{
		{ColumnName: collectionColumn, Operator: "=", Value: collectionRef.String()}, filter,
	})
	if err != nil {
		return nil, err
	}
	req := b.request(http.MethodGet, requestPath)
	req.QueryParams = map[string][]string{
		"query": {string(query)}, "page[size]": {fmt.Sprint(limit)},
		"included_relations": {"content"}, "sort": {"rpath"},
	}
	_, response, err := b.cruds[table].PaginatedFindAllWithTransaction(req, tx)
	if err != nil {
		return nil, davResourceError(err)
	}
	models, ok := response.Result().([]api2go.Api2GoModel)
	if !ok {
		return nil, fmt.Errorf("DAV sync resource list returned an invalid result")
	}
	visible := make(map[string]map[string]interface{}, len(models))
	for _, model := range models {
		row := model.GetAttributes()
		row["reference_id"] = model.GetID()
		visible[StringOrEmpty(row["rpath"])] = row
	}
	return visible, nil
}

func (b *DaptinDAVBackend) davSyncInitialRows(table, collectionTable, requestPath, collectionColumn string, collectionRef daptinid.DaptinReferenceId,
	after string, limit int, tx *sqlx.Tx) (map[string]map[string]interface{}, string, bool, error) {
	collectionID, err := GetReferenceIdToIdWithTransaction(collectionTable, collectionRef, tx)
	if err != nil {
		return nil, "", false, err
	}
	filters := []goqu.Ex{{collectionColumn: collectionID}}
	if after != "" {
		filters = append(filters, goqu.Ex{"rpath": goqu.Op{"gt": after}})
	}
	candidates, err := GetLimitedOrderedRowsWithTransaction(table, []string{"reference_id", "rpath"}, "rpath", tx, uint(limit+1), filters...)
	if err != nil {
		return nil, "", false, err
	}
	more := len(candidates) > limit
	if more {
		candidates = candidates[:limit]
	}
	if len(candidates) == 0 {
		return map[string]map[string]interface{}{}, after, false, nil
	}
	refs := make([]interface{}, 0, len(candidates))
	for _, candidate := range candidates {
		ref := daptinid.InterfaceToDIR(candidate["reference_id"])
		if ref == daptinid.NullReferenceId {
			return nil, "", false, fmt.Errorf("invalid DAV sync candidate reference")
		}
		refs = append(refs, ref.String())
	}
	lastPath := StringOrEmpty(candidates[len(candidates)-1]["rpath"])
	visible, err := b.davSyncVisibleRows(table, requestPath, collectionColumn, collectionRef,
		Query{ColumnName: "reference_id", Operator: "in", Value: refs}, limit, tx)
	return visible, lastPath, more, err
}

func (b *DaptinDAVBackend) davSyncPage(table, collectionTable, collectionColumn, requestPath string,
	collectionRef daptinid.DaptinReferenceId, previous davSyncCheckpoint, head int64, limit int, tx *sqlx.Tx) (
	map[string]map[string]interface{}, []string, davSyncCheckpoint, bool, error) {
	if previous.Phase == "initial" {
		visible, lastPath, more, err := b.davSyncInitialRows(table, collectionTable, requestPath, collectionColumn,
			collectionRef, previous.Path, limit, tx)
		if err != nil {
			return nil, nil, davSyncCheckpoint{}, false, err
		}
		paths := make([]string, 0, len(visible))
		for member := range visible {
			paths = append(paths, member)
		}
		sort.Strings(paths)
		next := previous
		next.Path = lastPath
		if !more {
			next.Phase = "changes"
			next.Path = ""
			next.Cursor = previous.Base
			more = head > previous.Base
			if !more {
				next.Phase = "steady"
				next.Cursor = head
			}
		}
		return visible, paths, next, more, nil
	}
	next := previous
	if len(next.Pending) == 0 {
		changes, _, err := b.davLogBatch(collectionRef, previous.Cursor, head, 1000, tx)
		if err != nil {
			return nil, nil, davSyncCheckpoint{}, false, err
		}
		latest := make(map[string]davLogChange, len(changes))
		for _, change := range changes {
			latest[change.path] = change
		}
		paths := make([]string, 0, len(latest))
		for member := range latest {
			paths = append(paths, member)
		}
		sort.Strings(paths)
		for _, member := range paths {
			change := latest[member]
			next.Pending = append(next.Pending, davSyncPending{Path: member, Removed: change.removed, ReadGrant: change.readGrant})
		}
		if len(changes) != 0 {
			next.Cursor = changes[len(changes)-1].revision
		} else {
			next.Cursor = head
		}
	}
	page := next.Pending
	if len(page) > limit {
		page = page[:limit]
	}
	next.Pending = next.Pending[len(page):]
	paths := make([]string, 0, len(page))
	for _, change := range page {
		paths = append(paths, change.Path)
	}
	visible := make(map[string]map[string]interface{})
	if len(paths) != 0 {
		selected, err := b.davSyncVisibleRows(table, requestPath, collectionColumn, collectionRef,
			Query{ColumnName: "rpath", Operator: "in", Value: paths}, limit, tx)
		if err != nil {
			return nil, nil, davSyncCheckpoint{}, false, err
		}
		visible = selected
	}
	objectCRUD := b.cruds[table]
	tableGrant := objectCRUD.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", table, tx)
	canReadTable := tableGrant.CanRead(b.sessionUser.UserReferenceId, b.sessionUser.Groups, objectCRUD.AdministratorGroupId)
	filtered := paths[:0]
	for _, change := range page {
		member := change.Path
		if _, ok := visible[member]; ok {
			filtered = append(filtered, member)
			continue
		}
		if canReadTable && change.Removed && change.ReadGrant.CanRead(b.sessionUser.UserReferenceId,
			b.sessionUser.Groups, objectCRUD.AdministratorGroupId) {
			filtered = append(filtered, member)
		}
	}
	next.Phase = "steady"
	more := len(next.Pending) != 0 || next.Cursor < head
	return visible, filtered, next, more, nil
}

func (b *DaptinDAVBackend) loadDAVSyncState(collectionRef daptinid.DaptinReferenceId, token string, tx *sqlx.Tx) (davSyncCheckpoint, error) {
	rows, _, err := b.cruds["dav_sync"].GetRowsByWhereClauseWithTransaction("dav_sync", nil, tx, goqu.Ex{"token": token})
	if err != nil {
		return davSyncCheckpoint{}, err
	}
	segments := make(map[int64]string, len(rows))
	for _, row := range rows {
		expires, err := ResourceRowInt64(row["expires_at"])
		if err != nil {
			return davSyncCheckpoint{}, err
		}
		if daptinid.InterfaceToDIR(row["collection_reference"]) != collectionRef ||
			daptinid.InterfaceToDIR(row["user_account_id"]) != b.sessionUser.UserReferenceId ||
			expires <= time.Now().Unix() {
			return davSyncCheckpoint{}, errInvalidDAVSyncToken
		}
		segment, err := ResourceRowInt64(row["segment"])
		if err != nil || segment < 0 {
			return davSyncCheckpoint{}, errInvalidDAVSyncToken
		}
		if _, duplicate := segments[segment]; duplicate {
			return davSyncCheckpoint{}, errInvalidDAVSyncToken
		}
		segments[segment] = StringOrEmpty(row["state"])
	}
	if len(segments) == 0 {
		return davSyncCheckpoint{}, errInvalidDAVSyncToken
	}
	var packed strings.Builder
	for i := range len(segments) {
		part, present := segments[int64(i)]
		if !present {
			return davSyncCheckpoint{}, errInvalidDAVSyncToken
		}
		packed.WriteString(part)
	}
	compressed, err := base64.RawStdEncoding.DecodeString(packed.String())
	if err != nil {
		return davSyncCheckpoint{}, errInvalidDAVSyncToken
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return davSyncCheckpoint{}, errInvalidDAVSyncToken
	}
	encoded, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil || closeErr != nil {
		return davSyncCheckpoint{}, errInvalidDAVSyncToken
	}
	var state davSyncCheckpoint
	if err := encodingjson.Unmarshal(encoded, &state); err != nil {
		return davSyncCheckpoint{}, errInvalidDAVSyncToken
	}
	if state.Access == "" || state.Cursor < 0 || state.Base < 0 ||
		(state.Phase != "initial" && state.Phase != "changes" && state.Phase != "steady") {
		return davSyncCheckpoint{}, errInvalidDAVSyncToken
	}
	return state, nil
}

func (b *DaptinDAVBackend) saveDAVSyncState(collectionRef daptinid.DaptinReferenceId, state davSyncCheckpoint, tx *sqlx.Tx) (string, error) {
	if err := b.pruneDAVSyncState(collectionRef, tx); err != nil {
		return "", err
	}
	if err := b.pruneDAVLog(collectionRef, tx); err != nil {
		return "", err
	}
	encoded, err := encodingjson.Marshal(state)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(collectionRef.String() + "\x00" + b.sessionUser.UserReferenceId.String() + "\x00" + string(encoded)))
	token := "urn:daptin:dav-sync:" + hex.EncodeToString(digest[:])
	refs, err := GetReferenceIdByWhereClauseWithTransaction("dav_sync", tx, goqu.Ex{"token": token})
	if err != nil {
		return "", err
	}
	if len(refs) != 0 {
		row, _, err := b.cruds["dav_sync"].GetSingleRowByReferenceIdWithTransaction("dav_sync", refs[0], nil, tx)
		if err != nil {
			return "", err
		}
		expires, err := ResourceRowInt64(row["expires_at"])
		if err != nil {
			return "", err
		}
		if expires > time.Now().Unix() {
			return token, nil
		}
		for _, ref := range refs {
			if _, err := b.cruds["dav_sync"].deleteAfterAuthorizationWithTransaction(ref,
				b.request(http.MethodDelete, "/api/dav_sync/"+ref.String()), tx); err != nil {
				return "", davResourceError(err)
			}
		}
	}
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(encoded); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	packed := base64.RawStdEncoding.EncodeToString(compressed.Bytes())
	expires := time.Now().Add(davSyncLifetime).Unix()
	for offset, segment := 0, 0; offset < len(packed); offset, segment = offset+davSyncSegmentSize, segment+1 {
		end := min(offset+davSyncSegmentSize, len(packed))
		model := api2go.NewApi2GoModelWithData("dav_sync", nil, int64(b.cruds["dav_sync"].TableInfo().DefaultPermission), nil,
			map[string]interface{}{"token": token, "segment": segment, "state": packed[offset:end], "expires_at": expires, "collection_reference": collectionRef.String()})
		if _, err := b.cruds["dav_sync"].createWithoutFilterAfterAuthorization(model,
			b.request(http.MethodPost, "/api/dav_sync"), tx); err != nil {
			return "", davResourceError(err)
		}
	}
	return token, nil
}

func (b *DaptinDAVBackend) pruneDAVSyncState(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("dav_sync", tx, 100,
		goqu.Ex{"collection_reference": collectionRef.String(), "expires_at": goqu.Op{"lt": time.Now().Unix()}})
	if err != nil {
		return err
	}
	for _, ref := range refs {
		if _, err := b.cruds["dav_sync"].deleteAfterAuthorizationWithTransaction(ref,
			b.request(http.MethodDelete, fmt.Sprintf("/api/dav_sync/%s", ref)), tx); err != nil {
			return davResourceError(err)
		}
	}
	return nil
}

func (b *DaptinDAVBackend) deleteDAVSyncState(collectionRef daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	for {
		refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction("dav_sync", tx, 100, goqu.Ex{"collection_reference": collectionRef.String()})
		if err != nil {
			return err
		}
		if len(refs) == 0 {
			return nil
		}
		for _, ref := range refs {
			if _, err := b.cruds["dav_sync"].deleteAfterAuthorizationWithTransaction(ref,
				b.request(http.MethodDelete, fmt.Sprintf("/api/dav_sync/%s", ref)), tx); err != nil {
				return davResourceError(err)
			}
		}
	}
}
