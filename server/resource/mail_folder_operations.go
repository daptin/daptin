package resource

import (
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/jmoiron/sqlx"
)

// AllocateMailBoxValidity uses the account's durable generation counter under
// its creation lock. The epoch floor also avoids legacy second-resolution
// UIDVALIDITY reuse when an old mailbox is deleted/recreated immediately.
func (dbResource *DbResource) AllocateMailBoxValidity(accountID int64, tx *sqlx.Tx) (uint32, error) {
	if err := dbResource.Cruds["mail_account"].LockMailAccountForMailboxCreation(accountID, tx); err != nil {
		return 0, err
	}
	query, args, err := statementbuilder.Squirrel.Select("next_uidvalidity").From("mail_account").Prepared(true).Where(goqu.Ex{"id": accountID}).ToSQL()
	if err != nil {
		return 0, err
	}
	var generation int64
	if err = tx.QueryRowx(query, args...).Scan(&generation); err != nil {
		return 0, err
	}
	floor := time.Now().Unix() + 1
	if generation < floor {
		generation = floor
	}
	if generation >= math.MaxUint32 {
		return 0, mailDraftError(409, "mailbox generation limit reached")
	}
	query, args, err = statementbuilder.Squirrel.Update("mail_account").Prepared(true).Set(goqu.Record{"next_uidvalidity": generation + 1}).Where(goqu.Ex{"id": accountID}).ToSQL()
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(query, args...); err != nil {
		return 0, err
	}
	return uint32(generation), nil
}
func (dbResource *DbResource) CreateMailFolderWithTransaction(accountRef, name string, active *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	if err := validateMailFolderName(name); err != nil {
		return nil, err
	}
	if strings.EqualFold(name, "INBOX") {
		name = "INBOX"
	}
	_, caller, err := dbResource.MailWriteAccount(accountRef, active, tx)
	if err != nil {
		return nil, err
	}
	ref, err := mailReference(accountRef)
	if err != nil {
		return nil, err
	}
	numeric, err := dbResource.GetReferenceIdToId("mail_account", ref, tx)
	if err != nil {
		return nil, err
	}
	if _, err = dbResource.GetMailAccountBox(numeric, name, tx); err == nil {
		return nil, mailDraftError(409, "folder already exists")
	}
	parts := strings.Split(name, "/")
	for i := range parts {
		path := strings.Join(parts[:i+1], "/")
		if _, err = dbResource.GetMailAccountBox(numeric, path, tx); err == nil {
			continue
		}
		if _, err = dbResource.CreateMailAccountBox(accountRef, caller, path, tx); err != nil {
			return nil, err
		}
	}
	return dbResource.GetMailAccountBox(numeric, name, tx)
}
func (dbResource *DbResource) renameMailHierarchy(accountID int64, oldName, newName string, caller *auth.SessionUser, tx *sqlx.Tx) error {
	if strings.HasPrefix(newName, oldName+"/") {
		return mailDraftError(409, "a folder cannot be moved inside itself")
	}
	rows, err := dbResource.Cruds["mail_box"].GetAllObjectsWithWhereWithTransaction("mail_box", tx, goqu.Ex{"mail_account_id": accountID})
	if err != nil {
		return err
	}
	moving := map[string]bool{}
	names := map[string]bool{}
	for _, row := range rows {
		name := StringOrEmpty(row["name"])
		names[name] = true
		if name == oldName || strings.HasPrefix(name, oldName+"/") {
			moving[name] = true
		}
	}
	for old := range moving {
		next := newName + strings.TrimPrefix(old, oldName)
		if err = validateMailFolderName(next); err != nil {
			return err
		}
		if names[next] && !moving[next] {
			return mailDraftError(409, "target folder already exists")
		}
	}
	for _, row := range rows {
		old := StringOrEmpty(row["name"])
		if !moving[old] {
			continue
		}
		ref := daptinid.InterfaceToDIR(row["reference_id"])
		if err = dbResource.mailRowPermission("mail_box", ref, caller, false, tx); err != nil {
			return err
		}
		crud := dbResource.Cruds["mail_box"]
		original, err := crud.GetReferenceIdToObjectWithTransaction("mail_box", ref, tx)
		if err != nil {
			return err
		}
		if original["version"] != row["version"] {
			return mailDraftError(409, "folder changed during rename")
		}
		model := api2go.NewApi2GoModelWithData("mail_box", nil, 0, nil, original)
		model.SetID(ref.String())
		model.SetAttributes(map[string]interface{}{"name": newName + strings.TrimPrefix(old, oldName)})
		request := mailResourceRequest(http.MethodPatch, caller)
		request.PlainRequest.URL.Path = "/api/mail_box"
		if _, err = crud.updateAfterAuthorizationWithTransaction(model, request, tx); err != nil {
			return err
		}
	}
	// Create any missing superior names after collision checks. The final leaf
	// already exists, so only parents enter the shared creation routine.
	accountRef, err := GetIdToReferenceIdWithTransaction("mail_account", accountID, tx)
	if err != nil {
		return err
	}
	parts := strings.Split(newName, "/")
	for i := 0; i < len(parts)-1; i++ {
		name := strings.Join(parts[:i+1], "/")
		if _, err = dbResource.GetMailAccountBox(accountID, name, tx); err != nil {
			if _, err = dbResource.CreateMailAccountBox(accountRef.String(), caller, name, tx); err != nil {
				return err
			}
		}
	}
	return nil
}
func mailFolderState(row map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{"exists": true, "reference_id": daptinid.InterfaceToDIR(row["reference_id"]).String(), "name": row["name"], "version": row["version"], "uidvalidity": row["uidvalidity"]}
}
func (dbResource *DbResource) MailFolderOperation(input MailFolderOperation, active *auth.SessionUser, operation string, tx *sqlx.Tx) (map[string]interface{}, error) {
	account, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return nil, err
	}
	accountID, err := dbResource.GetReferenceIdToId("mail_account", daptinid.InterfaceToDIR(account["reference_id"]), tx)
	if err != nil {
		return nil, err
	}
	if operation == "create" {
		row, err := dbResource.CreateMailFolderWithTransaction(input.AccountRef, input.Name, caller, tx)
		if err != nil {
			return nil, err
		}
		return mailFolderState(row), nil
	}
	ref, err := mailReference(input.FolderRef)
	if err != nil {
		return nil, err
	}
	row, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", ref, nil, tx)
	if err != nil {
		return nil, err
	}
	if daptinid.InterfaceToDIR(row["mail_account_id"]) != daptinid.InterfaceToDIR(account["reference_id"]) {
		return nil, mailDraftError(403, "folder is outside this mail account")
	}
	version, _ := ResourceRowInt64(row["version"])
	if version != input.ExpectedVersion || version < 1 {
		return nil, mailDraftError(409, "folder changed; refresh before another operation")
	}
	name := StringOrEmpty(row["name"])
	if reservedMailFolder(name) {
		return nil, mailDraftError(409, "Office system folders cannot be renamed or deleted")
	}
	if operation == "rename" {
		if err = dbResource.Cruds["mail_box"].RenameMailAccountBoxWithTransaction(accountID, name, input.Name, caller, tx); err != nil {
			return nil, err
		}
		latest, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", ref, nil, tx)
		if err != nil {
			return nil, err
		}
		return mailFolderState(latest), nil
	}
	if operation != "delete" {
		return nil, mailDraftError(400, "unsupported folder operation")
	}
	numeric, err := dbResource.GetReferenceIdToId("mail_box", ref, tx)
	if err != nil {
		return nil, err
	}
	query, args, err := statementbuilder.Squirrel.Select(goqu.COUNT("id")).From("mail").Prepared(true).Where(goqu.Ex{"mail_box_id": numeric}).ToSQL()
	if err != nil {
		return nil, err
	}
	var count int64
	if err = tx.QueryRowx(query, args...).Scan(&count); err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, mailDraftError(409, "move or delete the folder's messages before deleting the folder")
	}
	if err = dbResource.DeleteMailAccountBoxWithTransaction(accountID, name, caller, tx); err != nil {
		return nil, err
	}
	return map[string]interface{}{"exists": false, "reference_id": input.FolderRef}, nil
}
