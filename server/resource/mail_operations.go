package resource

import (
	"errors"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/daptin/daptin/server/statementbuilder"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type MailMessageOperation struct {
	AccountRef      string `json:"account_ref"`
	MessageRef      string `json:"message_ref"`
	ExpectedBoxRef  string `json:"expected_box_ref"`
	ExpectedVersion int64  `json:"expected_version"`
	DestinationRef  string `json:"destination_ref"`
	Seen            *bool  `json:"seen"`
	Flagged         *bool  `json:"flagged"`
}
type MailFolderOperation struct {
	AccountRef      string `json:"account_ref"`
	FolderRef       string `json:"folder_ref"`
	ExpectedVersion int64  `json:"expected_version"`
	Name            string `json:"name"`
}

func mailReference(value string) (daptinid.DaptinReferenceId, error) {
	id, err := uuid.Parse(value)
	if err != nil {
		return daptinid.NullReferenceId, mailDraftError(400, "invalid mail resource reference")
	}
	return daptinid.DaptinReferenceId(id), nil
}
func (dbResource *DbResource) lockMailMessage(ref daptinid.DaptinReferenceId, tx *sqlx.Tx) error {
	if isSQLiteDriver(tx.DriverName()) {
		return nil
	} // The account write lock owns SQLite's writer slot.
	query, args, err := statementbuilder.Squirrel.Select("id").From("mail").Prepared(true).Where(goqu.Ex{"reference_id": ref[:]}).ForUpdate(goqu.Wait).ToSQL()
	if err != nil {
		return err
	}
	var id int64
	return tx.QueryRowx(query, args...).Scan(&id)
}
func (dbResource *DbResource) writableMailRow(ref daptinid.DaptinReferenceId, active *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, *auth.SessionUser, error) {
	caller, err := dbResource.MailSessionUser(active, tx)
	if err != nil {
		return nil, nil, err
	}
	row, err := dbResource.readMailMetadataForPersistedSession(ref, caller, tx)
	if err != nil {
		return nil, nil, err
	}
	box, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), nil, tx)
	if err != nil {
		return nil, nil, err
	}
	_, caller, err = dbResource.MailWriteAccount(daptinid.InterfaceToDIR(box["mail_account_id"]).String(), caller, tx)
	if err != nil {
		return nil, nil, err
	}
	if err = dbResource.lockMailMessage(ref, tx); err != nil {
		return nil, nil, err
	}
	// Read again after the account lock so incremental changes start from current state.
	row, err = dbResource.readMailMetadataForPersistedSession(ref, caller, tx)
	if err != nil {
		return nil, nil, err
	}
	currentBox, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), nil, tx)
	if err != nil {
		return nil, nil, err
	}
	if daptinid.InterfaceToDIR(currentBox["mail_account_id"]) != daptinid.InterfaceToDIR(box["mail_account_id"]) {
		return nil, nil, mailDraftError(409, "message moved to another account; refresh before another operation")
	}
	if err = dbResource.mailRowPermission("mail", ref, caller, false, tx); err != nil {
		return nil, nil, err
	}
	if err = dbResource.mailRowPermission("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), caller, false, tx); err != nil {
		return nil, nil, err
	}
	return row, caller, nil
}
func (dbResource *DbResource) mailRowPermission(table string, ref daptinid.DaptinReferenceId, caller *auth.SessionUser, deleting bool, tx *sqlx.Tx) error {
	crud := dbResource.Cruds[table]
	tableGrant := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", table, tx)
	grant := GetObjectPermissionByReferenceIdWithTransaction(table, ref, tx)
	allowed := grant.CanUpdate(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId)
	if deleting {
		allowed = grant.CanDelete(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId)
	}
	if !tableGrant.CanPeek(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) || !grant.CanRead(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) || !allowed {
		return mailDraftError(403, "mail operation permission denied")
	}
	return nil
}
func (dbResource *DbResource) updateMailOperationRow(row map[string]interface{}, changes map[string]interface{}, caller *auth.SessionUser, tx *sqlx.Tx) error {
	ref := daptinid.InterfaceToDIR(row["reference_id"])
	original, err := dbResource.GetReferenceIdToObjectWithTransaction("mail", ref, tx)
	if err != nil {
		return err
	}
	if original["version"] != row["version"] {
		return mailDraftError(409, "message changed during operation")
	}
	model := api2go.NewApi2GoModelWithData("mail", nil, 0, nil, original)
	model.SetID(ref.String())
	model.SetAttributes(changes)
	_, err = dbResource.updateAfterAuthorizationWithTransaction(model, mailResourceRequest(http.MethodPatch, caller), tx)
	if errors.Is(err, ErrVersionConflict) {
		return mailDraftError(409, "message changed during operation")
	}
	return err
}
func canonicalMailFlags(flags []string) []string {
	system := map[string]string{"\\seen": imap.SeenFlag, "\\recent": imap.RecentFlag, "\\deleted": imap.DeletedFlag, "\\draft": imap.DraftFlag, "\\answered": imap.AnsweredFlag, "\\flagged": imap.FlaggedFlag}
	result := []string{}
	seen := map[string]bool{}
	for _, flag := range flags {
		flag = strings.TrimSpace(flag)
		key := strings.ToLower(flag)
		if flag == "" || seen[key] {
			continue
		}
		if standard, ok := system[key]; ok {
			flag = standard
		}
		seen[key] = true
		result = append(result, flag)
	}
	return result
}
func mailFlagAttributes(flags []string) map[string]interface{} {
	flags = canonicalMailFlags(flags)
	if HasFlag(flags, imap.DeletedFlag) {
		flags = backendutil.UpdateFlags(flags, imap.RemoveFlags, []string{imap.RecentFlag})
		flags = backendutil.UpdateFlags(flags, imap.AddFlags, []string{imap.SeenFlag})
	}
	return map[string]interface{}{"flags": strings.Join(flags, ","), "seen": HasFlag(flags, imap.SeenFlag), "recent": HasFlag(flags, imap.RecentFlag), "deleted": HasFlag(flags, imap.DeletedFlag)}
}

// MutateMailFlags is shared by native STORE/FETCH and browser updates. Resolve
// persisted permissions and lock the account before merging incremental flags.
func (dbResource *DbResource) MutateMailFlags(ref daptinid.DaptinReferenceId, operation imap.FlagsOp, flags []string, active *auth.SessionUser, tx *sqlx.Tx) error {
	row, caller, err := dbResource.writableMailRow(ref, active, tx)
	if err != nil {
		return err
	}
	current := canonicalMailFlags(strings.Split(StringOrEmpty(row["flags"]), ","))
	values := []string{}
	for _, flag := range canonicalMailFlags(flags) {
		if !strings.EqualFold(flag, imap.RecentFlag) {
			values = append(values, flag)
		}
	}
	next := backendutil.UpdateFlags(current, operation, values)
	if operation == imap.SetFlags && HasFlag(current, imap.RecentFlag) {
		next = append(next, imap.RecentFlag)
	}
	return dbResource.updateMailOperationRow(row, mailFlagAttributes(next), caller, tx)
}
func (dbResource *DbResource) MoveMailMessage(ref daptinid.DaptinReferenceId, destination map[string]interface{}, active *auth.SessionUser, tx *sqlx.Tx) error {
	row, caller, err := dbResource.writableMailRow(ref, active, tx)
	if err != nil {
		return err
	}
	destination, _, err = dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(destination["reference_id"]), nil, tx)
	if err != nil {
		return err
	}
	source, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), nil, tx)
	if err != nil {
		return err
	}
	if daptinid.InterfaceToDIR(source["mail_account_id"]) != daptinid.InterfaceToDIR(destination["mail_account_id"]) {
		return mailDraftError(403, "move must stay in the same mail account")
	}
	destRef := daptinid.InterfaceToDIR(destination["reference_id"])
	if err = dbResource.mailRowPermission("mail_box", destRef, caller, false, tx); err != nil {
		return err
	}
	if destRef == daptinid.InterfaceToDIR(row["mail_box_id"]) {
		return nil
	}
	boxID, err := dbResource.GetReferenceIdToId("mail_box", destRef, tx)
	if err != nil {
		return err
	}
	uid, err := dbResource.Cruds["mail_box"].AllocateMailBoxUid(boxID, tx)
	if err != nil {
		return err
	}
	flags := backendutil.UpdateFlags(canonicalMailFlags(strings.Split(StringOrEmpty(row["flags"]), ",")), imap.RemoveFlags, []string{imap.DeletedFlag, imap.RecentFlag})
	changes := mailFlagAttributes(flags)
	changes["mail_box_id"] = destRef
	changes["uid"] = uid
	return dbResource.updateMailOperationRow(row, changes, caller, tx)
}

// CheckMailDeletion protects linked delivery truth across browser deletion,
// native EXPUNGE and mailbox DELETE. Unknown/in-progress SMTP is never retried
// or silently erased by deleting a Sent copy.
func (dbResource *DbResource) CheckMailDeletion(ref daptinid.DaptinReferenceId, active *auth.SessionUser, tx *sqlx.Tx) error {
	_, caller, err := dbResource.writableMailRow(ref, active, tx)
	if err != nil {
		return err
	}
	if err = dbResource.mailRowPermission("mail", ref, caller, true, tx); err != nil {
		return err
	}
	numeric, err := dbResource.GetReferenceIdToId("mail", ref, tx)
	if err != nil {
		return err
	}
	query, args, err := statementbuilder.Squirrel.Select("sent", "retry_count", "last_error").From("outbox").Prepared(true).Where(goqu.Ex{"sent_mail_id": numeric}).ToSQL()
	if err != nil {
		return err
	}
	rows, err := tx.Queryx(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sent bool
		var retry int64
		var lastError *string
		if err = rows.Scan(&sent, &retry, &lastError); err != nil {
			return err
		}
		if !sent && (retry < 5 || lastError != nil && strings.HasPrefix(*lastError, OutboxSMTPUncertainPrefix)) {
			return mailDraftError(409, "queued or uncertain delivery must be reconciled before permanent deletion")
		}
	}
	return rows.Err()
}
func mailMessageState(row map[string]interface{}) map[string]interface{} {
	flags := canonicalMailFlags(strings.Split(StringOrEmpty(row["flags"]), ","))
	return map[string]interface{}{"exists": true, "reference_id": daptinid.InterfaceToDIR(row["reference_id"]).String(), "mail_box_id": daptinid.InterfaceToDIR(row["mail_box_id"]).String(), "version": row["version"], "uid": row["uid"], "flags": flags, "seen": HasFlag(flags, imap.SeenFlag), "flagged": HasFlag(flags, imap.FlaggedFlag), "deleted": HasFlag(flags, imap.DeletedFlag)}
}
func (dbResource *DbResource) MailMessageOperation(input MailMessageOperation, active *auth.SessionUser, operation string, tx *sqlx.Tx) (map[string]interface{}, error) {
	account, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return nil, err
	}
	ref, err := mailReference(input.MessageRef)
	if err != nil {
		return nil, err
	}
	found, err := dbResource.GetAllObjectsWithWhereWithTransaction("mail", tx, goqu.Ex{"reference_id": ref[:]})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		if operation == "state" {
			return map[string]interface{}{"exists": false, "reference_id": input.MessageRef}, nil
		}
		return nil, mailDraftError(409, "message no longer exists")
	}
	if operation != "state" {
		if err = dbResource.lockMailMessage(ref, tx); err != nil {
			return nil, err
		}
	}
	row, err := dbResource.readMailMetadataForPersistedSession(ref, caller, tx)
	if err != nil {
		return nil, err
	}
	box, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), nil, tx)
	if err != nil {
		return nil, err
	}
	if daptinid.InterfaceToDIR(box["mail_account_id"]) != daptinid.InterfaceToDIR(account["reference_id"]) {
		return nil, mailDraftError(403, "message is outside this mail account")
	}
	if operation == "state" {
		return mailMessageState(row), nil
	}
	version, _ := ResourceRowInt64(row["version"])
	if input.ExpectedVersion < 1 || input.ExpectedVersion != version || input.ExpectedBoxRef != daptinid.InterfaceToDIR(row["mail_box_id"]).String() {
		return nil, mailDraftError(409, "message changed or moved; refresh before another operation")
	}
	if operation == "flags" {
		if input.Seen == nil && input.Flagged == nil {
			return nil, mailDraftError(400, "choose a flag to update")
		}
		// Each incremental merge uses the same account lock; preserve native keywords.
		for _, flag := range []struct {
			value *bool
			name  string
		}{{input.Seen, imap.SeenFlag}, {input.Flagged, imap.FlaggedFlag}} {
			if flag.value == nil {
				continue
			}
			op := imap.FlagsOp(imap.RemoveFlags)
			if *flag.value {
				op = imap.AddFlags
			}
			if err = dbResource.MutateMailFlags(ref, op, []string{flag.name}, caller, tx); err != nil {
				return nil, err
			}
		}
	} else if operation == "delete" {
		if !strings.EqualFold(StringOrEmpty(box["name"]), "Trash") {
			return nil, mailDraftError(409, "move the message to Trash before permanent deletion")
		}
		if err = dbResource.CheckMailDeletion(ref, caller, tx); err != nil {
			return nil, err
		}
		if _, err = dbResource.deleteAfterAuthorizationWithTransaction(ref, mailResourceRequest(http.MethodDelete, caller), tx); err != nil {
			return nil, err
		}
		return map[string]interface{}{"exists": false, "reference_id": input.MessageRef}, nil
	} else {
		destination := input.DestinationRef
		if operation == "archive" || operation == "trash" {
			name := "Archive"
			if operation == "trash" {
				name = "Trash"
			}
			accountID, err := dbResource.GetReferenceIdToId("mail_account", daptinid.InterfaceToDIR(account["reference_id"]), tx)
			if err != nil {
				return nil, err
			}
			target, err := dbResource.GetMailAccountBox(accountID, name, tx)
			if err != nil {
				if _, err = dbResource.CreateMailAccountBox(input.AccountRef, caller, name, tx); err != nil {
					return nil, err
				}
				target, err = dbResource.GetMailAccountBox(accountID, name, tx)
				if err != nil {
					return nil, err
				}
			}
			destination = daptinid.InterfaceToDIR(target["reference_id"]).String()
		} else if operation != "move" {
			return nil, mailDraftError(400, "unsupported message operation")
		}
		destRef, err := mailReference(destination)
		if err != nil {
			return nil, err
		}
		target, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", destRef, nil, tx)
		if err != nil {
			return nil, err
		}
		if err = dbResource.MoveMailMessage(ref, target, caller, tx); err != nil {
			return nil, err
		}
	}
	latest, err := dbResource.readMailMetadataForPersistedSession(ref, caller, tx)
	if err != nil {
		return nil, err
	}
	return mailMessageState(latest), nil
}
func validateMailFolderName(name string) error {
	if name == "" || !utf8.ValidString(name) || utf8.RuneCountInString(name) > 100 || strings.TrimSpace(name) != name {
		return mailDraftError(422, "use a folder name of 1–100 characters")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return mailDraftError(422, "folder names cannot contain control characters")
		}
	}
	for _, part := range strings.Split(name, "/") {
		if part == "" || part == "." || part == ".." {
			return mailDraftError(422, "invalid folder hierarchy")
		}
	}
	return nil
}
func reservedMailFolder(name string) bool {
	for _, reserved := range []string{"INBOX", "Draft", "Sent", "Archive", "Trash", "Spam"} {
		if strings.EqualFold(name, reserved) {
			return true
		}
	}
	return false
}
