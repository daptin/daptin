package resource

import (
	"errors"
	"net/http"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/jmoiron/sqlx"
)

// MailSessionUser reloads the active account's persisted memberships. Action
// outcomes carry temporary administrator execution rights, which are not read
// grants on the account, mailbox, message, or MIME asset.
func (dbResource *DbResource) MailSessionUser(active *auth.SessionUser, tx *sqlx.Tx) (*auth.SessionUser, error) {
	if tx == nil {
		return nil, errors.New("mail operation requires an action transaction")
	}
	if active == nil || active.UserReferenceId == daptinid.NullReferenceId {
		return nil, api2go.NewHTTPError(errors.New("sign in to read mail"), "sign in to read mail", http.StatusUnauthorized)
	}
	return dbResource.MailAccountSessionUser(map[string]interface{}{USER_ACCOUNT_ID_COLUMN: active.UserReferenceId}, tx)
}

// ReadMailForSession checks the complete containing resource chain before any
// MIME asset is resolved. Action execution permission never substitutes for read.
func (dbResource *DbResource) ReadMailForSession(ref daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	caller, err := dbResource.MailSessionUser(caller, tx)
	if err != nil {
		return nil, err
	}
	return dbResource.readMailForPersistedSession(ref, caller, tx)
}

// caller has already been resolved from persisted memberships by the owning
// Mail operation. Keep parent-resource checks shared without reloading identity.
func (dbResource *DbResource) readMailForPersistedSession(ref daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	message, err := dbResource.readMailMetadataForPersistedSession(ref, caller, tx)
	if err != nil {
		return nil, err
	}
	message, _, err = dbResource.GetSingleRowByReferenceIdWithTransaction("mail", ref, map[string]bool{"mail": true}, tx)
	return message, err
}

func (dbResource *DbResource) readMailMetadataForPersistedSession(ref daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	message, err := dbResource.readMailResourceForSession("mail", ref, caller, tx)
	if err != nil {
		return nil, err
	}
	box, err := dbResource.readMailResourceForSession("mail_box", daptinid.InterfaceToDIR(message["mail_box_id"]), caller, tx)
	if err != nil {
		return nil, err
	}
	if _, err = dbResource.readMailResourceForSession("mail_account", daptinid.InterfaceToDIR(box["mail_account_id"]), caller, tx); err != nil {
		return nil, err
	}
	return message, nil
}

func (dbResource *DbResource) readMailResourceForSession(table string, ref daptinid.DaptinReferenceId, caller *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	crud := dbResource.Cruds[table]
	tablePermission := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", table, tx)
	rowPermission := GetObjectPermissionByReferenceIdWithTransaction(table, ref, tx)
	if ref == daptinid.NullReferenceId || !tablePermission.CanPeek(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) ||
		!rowPermission.CanRead(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) {
		return nil, api2go.NewHTTPError(errors.New("mail access denied"), "mail access denied", http.StatusForbidden)
	}
	row, _, err := crud.GetSingleRowByReferenceIdWithTransaction(table, ref, nil, tx)
	return row, err
}

// readMailBoxForSession checks the current principal and both containing
// resources; native selected-mailbox handles are not enduring read grants.
func (dbResource *DbResource) readMailBoxForSession(ref daptinid.DaptinReferenceId, active *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, *auth.SessionUser, error) {
	caller, err := dbResource.MailSessionUser(active, tx)
	if err != nil {
		return nil, nil, err
	}
	box, err := dbResource.readMailResourceForSession("mail_box", ref, caller, tx)
	if err != nil {
		return nil, nil, err
	}
	if _, err = dbResource.readMailResourceForSession("mail_account", daptinid.InterfaceToDIR(box["mail_account_id"]), caller, tx); err != nil {
		return nil, nil, err
	}
	return box, caller, nil
}
