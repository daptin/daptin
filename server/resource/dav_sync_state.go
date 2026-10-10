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
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

const (
	davSyncLifetime    = 30 * 24 * time.Hour
	davSyncSegmentSize = 32 * 1024
)

type davSyncCheckpoint struct {
	Members  map[string]string `json:"members"`
	Cursor   int64             `json:"cursor"`
	Reported map[string]int64  `json:"reported,omitempty"`
}

func davSyncPaths(previous davSyncCheckpoint, current map[string]string, logged map[string]davLogChange,
	limit int, visibleTombstone func(davLogChange) bool) ([]string, bool) {
	candidates := make(map[string]bool, len(previous.Members)+len(current)+len(logged))
	for member := range current {
		if previous.Members[member] != current[member] {
			candidates[member] = true
		}
	}
	for member := range previous.Members {
		if _, present := current[member]; !present {
			candidates[member] = true
		}
	}
	for member, change := range logged {
		if change.revision <= previous.Reported[member] {
			continue
		}
		if _, present := current[member]; present {
			candidates[member] = true
			continue
		}
		if _, previouslyVisible := previous.Members[member]; previouslyVisible || visibleTombstone(change) {
			candidates[member] = true
		}
	}
	paths := make([]string, 0, len(candidates))
	for member := range candidates {
		paths = append(paths, member)
	}
	sort.Strings(paths)
	more := len(paths) > limit
	if more {
		paths = paths[:limit]
	}
	return paths, more
}

func davNextCheckpoint(previous davSyncCheckpoint, current map[string]string,
	logged map[string]davLogChange, paths []string, head int64, more bool) davSyncCheckpoint {
	next := davSyncCheckpoint{Members: make(map[string]string, len(previous.Members)+len(paths)),
		Cursor: previous.Cursor, Reported: make(map[string]int64, len(previous.Reported)+len(paths))}
	for member, etag := range previous.Members {
		next.Members[member] = etag
	}
	for member, revision := range previous.Reported {
		next.Reported[member] = revision
	}
	for _, member := range paths {
		if etag, present := current[member]; present {
			next.Members[member] = etag
		} else {
			delete(next.Members, member)
		}
		if change, found := logged[member]; found {
			next.Reported[member] = change.revision
		}
	}
	if !more {
		next.Cursor = head
		next.Reported = nil
	}
	return next
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
