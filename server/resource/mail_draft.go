package resource

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type MailDraftRequest struct {
	AccountRef       string          `json:"account_ref"`
	DraftRef         string          `json:"draft_ref"`
	ComposeKey       string          `json:"compose_key"`
	WriteKey         string          `json:"write_key"`
	ExpectedRevision string          `json:"expected_revision"`
	Content          MailComposition `json:"content"`
}
type MailDraftResult struct {
	ReferenceID string      `json:"reference_id"`
	ComposeKey  string      `json:"compose_key"`
	WriteKey    string      `json:"write_key"`
	Revision    string      `json:"revision"`
	UID         interface{} `json:"uid"`
}

func mailDraftError(status int, message string) error {
	return api2go.NewHTTPError(errors.New(message), message, status)
}
func mailRevision(raw []byte) string { hash := sha256.Sum256(raw); return hex.EncodeToString(hash[:]) }

// MailWriteAccount reloads the active identity and enforces owner scope plus
// operation permission before locking the canonical account for draft changes.
func (dbResource *DbResource) MailWriteAccount(ref string, active *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, *auth.SessionUser, error) {
	caller, err := dbResource.MailSessionUser(active, tx)
	if err != nil {
		return nil, nil, err
	}
	id, err := uuid.Parse(ref)
	if err != nil {
		return nil, nil, mailDraftError(400, "invalid mail account reference")
	}
	crud := dbResource.Cruds["mail_account"]
	table := crud.GetObjectPermissionByWhereClauseWithTransaction("world", "table_name", "mail_account", tx)
	grant := GetObjectPermissionByReferenceIdWithTransaction("mail_account", daptinid.DaptinReferenceId(id), tx)
	if grant.UserId != caller.UserReferenceId || !table.CanPeek(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) ||
		!grant.CanRead(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) || !grant.CanUpdate(caller.UserReferenceId, caller.Groups, crud.AdministratorGroupId) {
		return nil, nil, mailDraftError(403, "mail account does not permit this write")
	}
	account, _, err := crud.GetSingleRowByReferenceIdWithTransaction("mail_account", daptinid.DaptinReferenceId(id), nil, tx)
	if err != nil {
		return nil, nil, err
	}
	numeric, err := crud.GetReferenceIdToId("mail_account", daptinid.DaptinReferenceId(id), tx)
	if err != nil {
		return nil, nil, err
	}
	if err = crud.LockMailAccountForMailboxCreation(numeric, tx); err != nil {
		return nil, nil, mailDraftError(409, "mail account is busy; reconcile before retrying")
	}
	return account, caller, nil
}

func mailResourceRequest(method string, caller *auth.SessionUser) api2go.Request {
	location, _ := url.Parse("/api/mail")
	return api2go.Request{PlainRequest: (&http.Request{Method: method, URL: location}).WithContext(context.WithValue(context.Background(), "user", caller))}
}

func (dbResource *DbResource) mailComposeRow(input MailDraftRequest, caller *auth.SessionUser, tx *sqlx.Tx, allowAdoption bool) (map[string]interface{}, error) {
	var ref daptinid.DaptinReferenceId
	if input.DraftRef != "" {
		id, err := uuid.Parse(input.DraftRef)
		if err != nil {
			return nil, mailDraftError(400, "invalid draft reference")
		}
		ref = daptinid.DaptinReferenceId(id)
	} else {
		rows, err := dbResource.GetAllObjectsWithWhereWithTransaction("mail", tx, goqu.Ex{"compose_key": input.ComposeKey})
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, nil
		}
		ref = daptinid.InterfaceToDIR(rows[0]["reference_id"])
	}
	row, err := dbResource.readMailForPersistedSession(ref, caller, tx)
	if err != nil {
		return nil, err
	}
	box, _, err := dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), nil, tx)
	if err != nil {
		return nil, err
	}
	if daptinid.InterfaceToDIR(box["mail_account_id"]).String() != input.AccountRef {
		return nil, mailDraftError(403, "draft does not belong to this compose operation")
	}
	if row["compose_key"] != input.ComposeKey {
		current, _ := row["compose_key"].(string)
		if !allowAdoption || input.DraftRef == "" || current != "" || input.ExpectedRevision == "" {
			return nil, mailDraftError(403, "draft does not belong to this compose operation")
		}
		// Native drafts have no browser operation identity. Claim only a named,
		// revision-checked draft; never reassign another compose operation.
		existing, err := dbResource.GetAllObjectsWithWhereWithTransaction("mail", tx, goqu.Ex{"compose_key": input.ComposeKey})
		if err != nil {
			return nil, err
		}
		if len(existing) != 0 {
			return nil, mailDraftError(409, "compose key already belongs to another message")
		}
	}

	return row, nil
}

func (dbResource *DbResource) SaveMailDraft(input MailDraftRequest, active *auth.SessionUser, tx *sqlx.Tx) (*MailDraftResult, error) {
	if _, err := uuid.Parse(input.ComposeKey); err != nil {
		return nil, mailDraftError(400, "invalid compose key")
	}
	if _, err := uuid.Parse(input.WriteKey); err != nil {
		return nil, mailDraftError(400, "invalid write key")
	}
	account, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return nil, err
	}
	row, err := dbResource.mailComposeRow(input, caller, tx, true)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(input.Content)
	if err != nil {
		return nil, err
	}
	contentHash := mailRevision(encoded)
	var original map[string]interface{}
	if row != nil {
		original, err = dbResource.GetReferenceIdToObjectWithTransaction("mail", daptinid.InterfaceToDIR(row["reference_id"]), tx)
		if err != nil {
			return nil, err
		}
		if original["version"] != row["version"] {
			return nil, mailDraftError(409, "draft changed during save")
		}
	}
	if row != nil {
		flags, _ := row["flags"].(string)
		if !HasFlag(strings.Split(flags, ","), imap.DraftFlag) || HasFlag(strings.Split(flags, ","), imap.DeletedFlag) {
			return nil, mailDraftError(409, "this message is no longer a draft")
		}
		raw, err := dbResource.MailColumnBytes("mail", "mail", row["mail"])
		if err != nil {
			return nil, err
		}
		if row["draft_write_key"] == input.WriteKey {
			if row["draft_write_revision"] != mailRevision(raw) {
				return nil, mailDraftError(409, "saved MIME changed after this write; reload before saving")
			}
			if row["draft_write_hash"] != contentHash {
				return nil, mailDraftError(409, "write key already belongs to different content")
			}
			return &MailDraftResult{ReferenceID: daptinid.InterfaceToDIR(row["reference_id"]).String(), ComposeKey: input.ComposeKey, WriteKey: input.WriteKey, Revision: mailRevision(raw), UID: row["uid"]}, nil
		}
		if input.ExpectedRevision == "" || mailRevision(raw) != input.ExpectedRevision {
			return nil, mailDraftError(409, "draft changed; reload before saving")
		}
		grant := GetObjectPermissionByReferenceIdWithTransaction("mail", daptinid.InterfaceToDIR(row["reference_id"]), tx)
		if !grant.CanUpdate(caller.UserReferenceId, caller.Groups, dbResource.AdministratorGroupId) {
			return nil, mailDraftError(403, "draft update denied")
		}
	} else if input.ExpectedRevision != "" {
		return nil, mailDraftError(409, "draft no longer exists")
	}
	sender, _ := account["username"].(string)
	mailbox, parseErr := composeMailbox(MailRecipient{Address: sender})
	if parseErr != nil {
		return nil, mailDraftError(422, "mail account has an invalid sender")
	}
	sender = mailbox.Address
	at := strings.LastIndex(sender, "@")
	if at < 0 {
		return nil, mailDraftError(422, "mail account has an invalid sender")
	}
	raw, err := ComposeDraftMail(sender, input.ComposeKey+sender[at:], time.Now().UTC(), input.Content)
	if err != nil {
		return nil, mailDraftError(422, err.Error())
	}
	accountID, err := dbResource.Cruds["mail_account"].GetReferenceIdToId("mail_account", daptinid.InterfaceToDIR(account["reference_id"]), tx)
	if err != nil {
		return nil, err
	}
	var box map[string]interface{}
	if row != nil {
		box, _, err = dbResource.Cruds["mail_box"].GetSingleRowByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(row["mail_box_id"]), nil, tx)
	} else {
		box, err = dbResource.GetMailAccountBox(accountID, "Draft", tx)
		if err != nil {
			if _, err = dbResource.CreateMailAccountBox(input.AccountRef, caller, "Draft", tx); err != nil {
				return nil, err
			}
			box, err = dbResource.GetMailAccountBox(accountID, "Draft", tx)
		}
	}
	if err != nil {
		return nil, err
	}
	grant := GetObjectPermissionByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(box["reference_id"]), tx)
	if !grant.CanUpdate(caller.UserReferenceId, caller.Groups, dbResource.AdministratorGroupId) {
		return nil, mailDraftError(403, "draft mailbox write denied")
	}
	flags := []string{imap.DraftFlag, imap.SeenFlag}
	if row != nil {
		current, _ := row["flags"].(string)
		flags = strings.Split(current, ",")
		if !HasFlag(flags, imap.SeenFlag) {
			flags = append(flags, imap.SeenFlag)
		}
	}
	if row == nil {
		row, err = dbResource.appendMailMessageWithTransaction(box, raw, flags, time.Now(), caller, tx)
		if err != nil {
			return nil, err
		}
	}
	ref := daptinid.InterfaceToDIR(row["reference_id"])
	uid, err := ResourceRowInt64(row["uid"])
	if err != nil {
		return nil, err
	}
	changes, err := dbResource.mailMessageAttributes(raw, box, uint32(uid))
	if err != nil {
		return nil, err
	}
	changes["recent"] = HasFlag(flags, imap.RecentFlag)
	changes["internal_date"] = row["internal_date"]
	changes["flags"] = strings.Join(flags, ",")
	changes["compose_key"] = input.ComposeKey
	changes["draft_write_key"] = input.WriteKey
	changes["draft_write_hash"] = contentHash
	changes["draft_write_revision"] = mailRevision(raw)
	changes["permission"] = int64(auth.UserCRUD | auth.UserExecute)
	if original == nil {
		original, err = dbResource.GetReferenceIdToObjectWithTransaction("mail", ref, tx)
		if err != nil {
			return nil, err
		}
	}
	model := api2go.NewApi2GoModelWithData("mail", nil, 0, nil, original)
	model.SetID(ref.String())
	model.SetAttributes(changes)
	if _, err = dbResource.updateAfterAuthorizationWithTransaction(model, mailResourceRequest(http.MethodPatch, caller), tx); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return nil, mailDraftError(409, "draft changed during save")
		}
		return nil, err
	}
	return &MailDraftResult{ReferenceID: ref.String(), ComposeKey: input.ComposeKey, WriteKey: input.WriteKey, Revision: mailRevision(raw), UID: uid}, nil
}

func (dbResource *DbResource) MailDraftStatus(input MailDraftRequest, active *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	_, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return nil, err
	}
	if _, err = uuid.Parse(input.ComposeKey); err != nil {
		return nil, mailDraftError(400, "invalid compose key")
	}
	input.DraftRef = "" // Reconcile by stable operation identity, even after a move.
	row, err := dbResource.mailComposeRow(input, caller, tx, false)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return map[string]interface{}{"exists": false, "compose_key": input.ComposeKey}, nil
	}
	raw, err := dbResource.MailColumnBytes("mail", "mail", row["mail"])
	if err != nil {
		return nil, err
	}
	flags, _ := row["flags"].(string)
	return map[string]interface{}{"exists": true, "reference_id": daptinid.InterfaceToDIR(row["reference_id"]).String(), "compose_key": input.ComposeKey, "write_key": row["draft_write_key"], "write_revision": row["draft_write_revision"], "revision": mailRevision(raw), "is_draft": HasFlag(strings.Split(flags, ","), imap.DraftFlag) && !HasFlag(strings.Split(flags, ","), imap.DeletedFlag)}, nil
}

func (dbResource *DbResource) DeleteMailDraft(input MailDraftRequest, active *auth.SessionUser, tx *sqlx.Tx) error {
	_, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return err
	}
	if _, err = uuid.Parse(input.ComposeKey); err != nil {
		return mailDraftError(400, "invalid compose key")
	}
	wanted := input.DraftRef
	input.DraftRef = ""
	row, err := dbResource.mailComposeRow(input, caller, tx, false)
	if err != nil {
		return err
	}
	if row == nil {
		return nil
	}
	ref := daptinid.InterfaceToDIR(row["reference_id"])
	if wanted != "" && wanted != ref.String() {
		return mailDraftError(409, "draft reference changed")
	}
	flags, _ := row["flags"].(string)
	if !HasFlag(strings.Split(flags, ","), imap.DraftFlag) {
		return mailDraftError(409, "only drafts can be discarded")
	}
	grant := GetObjectPermissionByReferenceIdWithTransaction("mail", ref, tx)
	if !grant.CanDelete(caller.UserReferenceId, caller.Groups, dbResource.AdministratorGroupId) {
		return mailDraftError(403, "draft deletion denied")
	}
	raw, err := dbResource.MailColumnBytes("mail", "mail", row["mail"])
	if err != nil {
		return err
	}
	if input.ExpectedRevision == "" || mailRevision(raw) != input.ExpectedRevision {
		return mailDraftError(409, "draft changed; reload before discarding")
	}
	_, err = dbResource.deleteAfterAuthorizationWithTransaction(ref, mailResourceRequest(http.MethodDelete, caller), tx)
	return err
}
