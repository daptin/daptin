package resource

import (
	"context"
	"errors"
	"net/http"
	"path"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/go-webdav"
	"github.com/daptin/go-webdav/carddav"
	"github.com/jmoiron/sqlx"
)

func (b *DaptinDAVBackend) addressBookEditAllowed(book map[string]interface{}, tx *sqlx.Tx, create bool) bool {
	crud := b.cruds[addressBookTable]
	grant := GetObjectPermissionByReferenceIdWithTransaction(addressBookTable, daptinid.InterfaceToDIR(book["reference_id"]), tx)
	user, groups, admin := b.sessionUser.UserReferenceId, b.sessionUser.Groups, crud.AdministratorGroupId
	if create {
		return grant.CanCreate(user, groups, admin) && grant.CanRefer(user, groups, admin)
	}
	return grant.CanUpdate(user, groups, admin)
}

func (b *DaptinDAVBackend) addressBookGroupGrants(book map[string]interface{}, tx *sqlx.Tx) (map[daptinid.DaptinReferenceId]auth.AuthPermission, error) {
	return b.davCollectionGroupGrants(addressBookTable, book, tx)
}

// PatchAddressBookProperties updates only collection metadata. The URL name
// remains stable when a client changes DAV:displayname.
func (b *DaptinDAVBackend) PatchAddressBookProperties(_ context.Context, requestPath string, changes []carddav.AddressBookPropertyUpdate) error {
	if _, _, err := b.addressBookPath(requestPath, false); err != nil {
		return err
	}
	tx, err := b.cruds[addressBookTable].Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	book, err := b.addressBookWithTransaction(requestPath, false, tx)
	if err != nil {
		return err
	}
	if !b.addressBookEditAllowed(book, tx, false) {
		return webdav.NewHTTPError(http.StatusForbidden, errors.New("address book metadata update denied"))
	}
	if err := b.lockAddressBook(book, tx); err != nil {
		return err
	}
	attrs := map[string]interface{}{}
	for _, change := range changes {
		value := change.Value
		if change.Remove {
			value = ""
		}
		switch {
		case change.Name.Space == "DAV:" && change.Name.Local == "displayname":
			attrs["display_name"] = value
		case change.Name.Space == "urn:ietf:params:xml:ns:carddav" && change.Name.Local == "addressbook-description":
			attrs["description"] = value
		default:
			return webdav.NewHTTPError(http.StatusForbidden, errors.New("protected address book property"))
		}
	}
	if len(attrs) == 0 {
		return webdav.NewHTTPError(http.StatusBadRequest, errors.New("no address book properties supplied"))
	}
	attrs["reference_id"] = daptinid.InterfaceToDIR(book["reference_id"]).String()
	model := api2go.NewApi2GoModelWithData(addressBookTable, nil, 0, nil, attrs)
	if _, err := b.cruds[addressBookTable].updateAfterAuthorizationWithTransaction(model,
		b.request(http.MethodPatch, path.Clean(requestPath)), tx); err != nil {
		return davResourceError(err)
	}
	return tx.Commit()
}
