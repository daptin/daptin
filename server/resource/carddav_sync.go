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
	owner, name, err := b.addressBookPath(requestPath, object)
	if err != nil {
		return nil, err
	}
	rows, err := b.calendarRowsWithTransaction(addressBookTable, requestPath, tx,
		Query{ColumnName: "name", Operator: "=", Value: name},
		Query{ColumnName: "user_account_id", Operator: "=", Value: owner.String()})
	return firstDAVObjectRow(rows, err)
}

func (b *DaptinDAVBackend) lockAddressBook(book map[string]interface{}, tx *sqlx.Tx) error {
	ref := daptinid.InterfaceToDIR(book["reference_id"])
	return b.cruds[addressBookTable].lockRowByWhereWithTransaction(tx, goqu.Ex{"reference_id": ref[:]})
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
	bookRef := daptinid.InterfaceToDIR(book["reference_id"])
	head, err := b.davLogHead(bookRef, tx)
	if err != nil {
		return "", err
	}
	access, err := b.davSyncAccess(addressBookTable, addressObjectTable, bookRef, tx)
	if err != nil {
		return "", err
	}
	token, err := b.saveDAVSyncState(bookRef, davSyncCheckpoint{Phase: "steady", Cursor: head, Access: access}, tx)
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
	access, err := b.davSyncAccess(addressBookTable, addressObjectTable, bookRef, tx)
	if err != nil {
		return nil, err
	}
	previous := davSyncCheckpoint{Phase: "initial", Cursor: head, Base: head, Access: access}
	if previousToken != "" {
		previous, err = b.loadDAVSyncState(bookRef, previousToken, tx)
		if errors.Is(err, errInvalidDAVSyncToken) {
			return nil, carddav.ErrInvalidSyncToken
		}
		if err != nil {
			return nil, err
		}
		if previous.Access != access || previous.Cursor > head {
			return nil, carddav.ErrInvalidSyncToken
		}
	}
	visible, paths, next, more, err := b.davSyncPage(addressObjectTable, addressBookTable, "address_book_id",
		requestPath, bookRef, previous, head, limit, tx)
	if err != nil {
		return nil, err
	}
	result := &carddav.AddressBookSyncResult{More: more, Changes: make([]carddav.AddressBookSyncChange, 0, len(paths))}
	for _, member := range paths {
		if row, present := visible[member]; present {
			object, err := b.addressObject(row)
			if err != nil {
				return nil, err
			}
			result.Changes = append(result.Changes, carddav.AddressBookSyncChange{Path: member, Object: &object})
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
