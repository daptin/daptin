package resource

import (
	"context"
	"errors"
	"net/http"

	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/carddav"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

func (b *DaptinDAVBackend) addressBookWithTransaction(requestPath string, object bool, tx *sqlx.Tx) (map[string]interface{}, error) {
	name, err := b.collectionName(requestPath, object)
	if err != nil {
		return nil, err
	}
	rows, err := b.rowsWithTransaction(addressBookTable, tx,
		goqu.Ex{"name": name, "user_account_id": b.sessionUser.UserId})
	return firstDAVObjectRow(rows, err)
}

func (b *DaptinDAVBackend) lockAddressBook(book map[string]interface{}, tx *sqlx.Tx) error {
	ref := daptinid.InterfaceToDIR(book["reference_id"])
	return b.cruds[addressBookTable].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": ref[:]})
}

func (b *DaptinDAVBackend) addressSyncState(book map[string]interface{}, tx *sqlx.Tx) (map[string]string, map[string]carddav.AddressObject, error) {
	bookRef := daptinid.InterfaceToDIR(book["reference_id"])
	bookID, err := GetReferenceIdToIdWithTransaction(addressBookTable, bookRef, tx)
	if err != nil {
		return nil, nil, err
	}
	refs, err := GetLimitedReferenceIdByWhereClauseWithTransaction(addressObjectTable, tx,
		maxDAVSyncObjects+1, goqu.Ex{"address_book_id": bookID})
	if err != nil {
		return nil, nil, err
	}
	if len(refs) > maxDAVSyncObjects {
		return nil, nil, webdav.NewHTTPError(http.StatusInsufficientStorage, errors.New("address book has too many objects to sync"))
	}
	rows, err := b.rowsWithTransaction(addressObjectTable, tx,
		goqu.Ex{"address_book_id": bookID, "user_account_id": b.sessionUser.UserId})
	if err != nil {
		return nil, nil, err
	}
	state := make(map[string]string, len(rows))
	objects := make(map[string]carddav.AddressObject, len(rows))
	for _, row := range rows {
		object, err := b.addressObject(row)
		if err != nil {
			return nil, nil, err
		}
		state[object.Path] = object.ETag
		objects[object.Path] = object
	}
	return state, objects, nil
}

func (b *DaptinDAVBackend) AddressBookSyncToken(_ context.Context, requestPath string) (string, error) {
	tx, err := b.cruds[addressBookTable].Connection().Beginx()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	book, err := b.addressBookWithTransaction(requestPath, false, tx)
	if err != nil {
		return "", err
	}
	if err := b.lockAddressBook(book, tx); err != nil {
		return "", err
	}
	state, _, err := b.addressSyncState(book, tx)
	if err != nil {
		return "", err
	}
	bookRef := daptinid.InterfaceToDIR(book["reference_id"])
	head, err := b.davLogHead(bookRef, tx)
	if err != nil {
		return "", err
	}
	token, err := b.saveDAVSyncState(bookRef, davSyncCheckpoint{Members: state, Cursor: head}, tx)
	if err != nil {
		return "", err
	}
	return token, tx.Commit()
}

func (b *DaptinDAVBackend) SyncAddressObjects(_ context.Context, requestPath, previousToken string, limit int) (*carddav.AddressBookSyncResult, error) {
	if limit < 1 || limit > 1000 {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, errors.New("invalid address book sync limit"))
	}
	tx, err := b.cruds[addressBookTable].Connection().Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	book, err := b.addressBookWithTransaction(requestPath, false, tx)
	if err != nil {
		return nil, err
	}
	if err := b.lockAddressBook(book, tx); err != nil {
		return nil, err
	}
	bookRef := daptinid.InterfaceToDIR(book["reference_id"])
	head, err := b.davLogHead(bookRef, tx)
	if err != nil {
		return nil, err
	}
	previous := davSyncCheckpoint{Members: make(map[string]string), Cursor: head}
	if previousToken != "" {
		previous, err = b.loadDAVSyncState(bookRef, previousToken, tx)
		if errors.Is(err, errInvalidDAVSyncToken) {
			return nil, carddav.ErrInvalidSyncToken
		}
		if err != nil {
			return nil, err
		}
	}
	if previous.Members == nil {
		previous.Members = make(map[string]string)
	}
	current, objects, err := b.addressSyncState(book, tx)
	if err != nil {
		return nil, err
	}
	logged, err := b.davLogChanges(bookRef, previous.Cursor, head, tx)
	if errors.Is(err, errInvalidDAVSyncToken) {
		return nil, carddav.ErrInvalidSyncToken
	}
	if err != nil {
		return nil, err
	}
	// CardDAV paths are restricted to this account's own address books.
	// A removed object recorded by its DAV write remains visible as a tombstone.
	paths, more := davSyncPaths(previous, current, logged, limit, func(change davLogChange) bool {
		return change.removed
	})
	next := davNextCheckpoint(previous, current, logged, paths, head, more)
	result := &carddav.AddressBookSyncResult{More: more, Changes: make([]carddav.AddressBookSyncChange, 0, len(paths))}
	for _, member := range paths {
		if object, present := objects[member]; present {
			copy := object
			result.Changes = append(result.Changes, carddav.AddressBookSyncChange{Path: member, Object: &copy})
		} else {
			result.Changes = append(result.Changes, carddav.AddressBookSyncChange{Path: member})
		}
	}
	result.Token, err = b.saveDAVSyncState(bookRef, next, tx)
	if err != nil {
		return nil, err
	}
	return result, tx.Commit()
}
