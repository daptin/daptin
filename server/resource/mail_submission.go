package resource

import (
	"bufio"
	"bytes"
	"database/sql"
	"errors"
	"github.com/daptin/daptin/server/statementbuilder"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/artpar/api2go/v2"
	"github.com/artpar/go-imap"
	"github.com/daptin/daptin/server/auth"
	daptinid "github.com/daptin/daptin/server/id"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-message"
	mailheader "github.com/emersion/go-message/mail"
	"github.com/emersion/go-message/textproto"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type MailSubmissionRequest struct {
	AccountRef       string `json:"account_ref"`
	ComposeKey       string `json:"compose_key"`
	SubmissionKey    string `json:"submission_key"`
	ExpectedRevision string `json:"expected_revision"`
}

// outgoingDraft preserves the saved MIME body byte-for-byte (including boundaries,
// transfer encodings and attachments). Only transport headers are replaced.
func outgoingDraft(sender string, raw []byte, at time.Time) ([]byte, []string, error) {
	if len(raw) > 25<<20 {
		return nil, nil, errors.New("message exceeds 25 MiB")
	}
	split := bytes.Index(raw, []byte("\r\n\r\n"))
	if split < 0 || split > 1<<20 {
		return nil, nil, errors.New("invalid or oversized message headers")
	}
	reader := bufio.NewReader(bytes.NewReader(raw))
	header, err := textproto.ReadHeader(reader)
	if err != nil {
		return nil, nil, err
	}
	h := mailheader.Header{Header: message.Header{Header: header}}
	from, err := composeMailbox(MailRecipient{Address: sender})
	if err != nil {
		return nil, nil, err
	}
	envelope := []string{}
	seen := map[string]bool{}
	count := 0
	for _, key := range []string{"To", "Cc", "Bcc"} {
		if !header.Has(key) {
			continue
		}
		// Reject duplicate recipient headers rather than silently dropping recipients.
		fields := header.FieldsByKey(key)
		n := 0
		for fields.Next() {
			n++
		}
		if n != 1 {
			return nil, nil, errors.New("duplicate recipient header")
		}
		addresses, err := h.AddressList(key)
		if err != nil {
			return nil, nil, errors.New("invalid recipient header")
		}
		for _, address := range addresses {
			normalized, err := composeMailbox(MailRecipient{Name: address.Name, Address: address.Address})
			if err != nil {
				return nil, nil, err
			}
			count++
			if count > 100 {
				return nil, nil, errors.New("recipient count exceeds 100")
			}
			if !seen[normalized.Address] {
				seen[normalized.Address] = true
				envelope = append(envelope, normalized.Address)
			}
		}
	}
	if len(envelope) == 0 {
		return nil, nil, errors.New("at least one recipient is required")
	}
	for _, key := range []string{"Bcc", "Resent-Bcc", "Resent-From", "Resent-To", "Resent-Cc", "Resent-Sender", "Resent-Date", "Resent-Message-ID", "Sender", "Return-Path", "DKIM-Signature", "Authentication-Results", "Received"} {
		h.Del(key)
	}
	h.SetAddressList("From", []*mailheader.Address{from})
	h.SetDate(at)
	h.SetMessageID(uuid.NewString() + sender[strings.LastIndex(sender, "@"):])
	var output bytes.Buffer
	if err = textproto.WriteHeader(&output, h.Header.Header); err != nil {
		return nil, nil, err
	}
	if _, err = io.Copy(&output, reader); err != nil {
		return nil, nil, err
	}
	return output.Bytes(), envelope, nil
}

// QueueOutgoingMail is the shared legacy/structured send queue boundary. The
// sender's canonical ownership is retained on every outbox recipient; no action
// executor's temporary administrator membership enters this lifecycle.
func (dbResource *DbResource) QueueOutgoingMail(account map[string]interface{}, sentRef daptinid.DaptinReferenceId, raw []byte, recipients []string, tx *sqlx.Tx) ([]daptinid.DaptinReferenceId, error) {
	owner, err := dbResource.MailAccountSessionUser(account, tx)
	if err != nil {
		return nil, err
	}
	from, _ := account["username"].(string)
	fromAddress, err := composeMailbox(MailRecipient{Address: from})
	if err != nil {
		return nil, err
	}
	serverRef := daptinid.InterfaceToDIR(account["mail_server_id"])
	if serverRef == daptinid.NullReferenceId {
		return nil, errors.New("sender mail account has no mail server")
	}
	location, _ := url.Parse("/api/outbox")
	request := mailResourceRequest(http.MethodPost, owner)
	request.PlainRequest.URL = location
	refs := make([]daptinid.DaptinReferenceId, 0, len(recipients))
	for _, recipient := range recipients {
		address, err := composeMailbox(MailRecipient{Address: recipient})
		if err != nil {
			return nil, err
		}
		attrs := map[string]interface{}{
			"from_address": fromAddress.Address, "to_address": address.Address, "to_host": address.Address[strings.LastIndex(address.Address, "@")+1:],
			"mail_server_id": serverRef, "sent_mail_id": sentRef, "mail": dbResource.MailColumnValue("outbox", "mail", raw, "outgoing"),
			"sent": false, "retry_count": 0, "next_retry_at": time.Now(),
		}
		result, err := dbResource.Cruds["outbox"].createAfterAuthorizationWithTransaction(api2go.NewApi2GoModelWithData("outbox", nil, 0, nil, attrs), request, tx)
		if err != nil {
			return nil, err
		}
		refs = append(refs, daptinid.InterfaceToDIR(result.Result().(api2go.Api2GoModel).GetAttributes()["reference_id"]))
	}
	return refs, nil
}

// SubmitMailDraft queues exactly one saved revision while moving the canonical
// message into Sent with a new mailbox UID. Signing has no network side effects.
func (dbResource *DbResource) SubmitMailDraft(input MailSubmissionRequest, active *auth.SessionUser, tx *sqlx.Tx, sign func(string, []byte, *sqlx.Tx) ([]byte, error)) (map[string]interface{}, error) {
	for _, key := range []string{input.ComposeKey, input.SubmissionKey} {
		if _, err := uuid.Parse(key); err != nil {
			return nil, mailDraftError(400, "invalid submission identity")
		}
	}
	account, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return nil, err
	}
	row, err := dbResource.mailComposeRow(MailDraftRequest{AccountRef: input.AccountRef, ComposeKey: input.ComposeKey}, caller, tx, false)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, mailDraftError(409, "saved draft no longer exists")
	}
	if key, _ := row["submission_key"].(string); key != "" {
		if key != input.SubmissionKey || input.ExpectedRevision != row["submission_revision"] {
			return nil, mailDraftError(409, "compose operation already submitted")
		}
		return dbResource.mailSubmissionResult(row, tx)
	}
	flags, _ := row["flags"].(string)
	if !HasFlag(strings.Split(flags, ","), imap.DraftFlag) || HasFlag(strings.Split(flags, ","), imap.DeletedFlag) {
		return nil, mailDraftError(409, "message is no longer an active draft")
	}
	ref := daptinid.InterfaceToDIR(row["reference_id"])
	grant := GetObjectPermissionByReferenceIdWithTransaction("mail", ref, tx)
	if !grant.CanUpdate(caller.UserReferenceId, caller.Groups, dbResource.AdministratorGroupId) {
		return nil, mailDraftError(403, "draft submission denied")
	}
	original, err := dbResource.GetReferenceIdToObjectWithTransaction("mail", ref, tx)
	if err != nil {
		return nil, err
	}
	if original["version"] != row["version"] {
		return nil, mailDraftError(409, "draft changed during submission")
	}
	raw, err := dbResource.MailColumnBytes("mail", "mail", row["mail"])
	if err != nil {
		return nil, err
	}
	if input.ExpectedRevision == "" || mailRevision(raw) != input.ExpectedRevision {
		return nil, mailDraftError(409, "draft changed; reload before sending")
	}
	sender, _ := account["username"].(string)
	outgoing, envelope, err := outgoingDraft(sender, raw, time.Now().UTC())
	if err != nil {
		return nil, mailDraftError(422, err.Error())
	}
	outgoing, err = sign(sender[strings.LastIndex(sender, "@")+1:], outgoing, tx)
	if err != nil {
		return nil, mailDraftError(422, "sender signing configuration is unavailable")
	}
	accountID, err := dbResource.Cruds["mail_account"].GetReferenceIdToId("mail_account", daptinid.InterfaceToDIR(account["reference_id"]), tx)
	if err != nil {
		return nil, err
	}
	box, err := dbResource.GetMailAccountBox(accountID, "Sent", tx)
	if err != nil {
		if _, err = dbResource.CreateMailAccountBox(input.AccountRef, caller, "Sent", tx); err != nil {
			return nil, err
		}
		box, err = dbResource.GetMailAccountBox(accountID, "Sent", tx)
		if err != nil {
			return nil, err
		}
	}
	boxGrant := GetObjectPermissionByReferenceIdWithTransaction("mail_box", daptinid.InterfaceToDIR(box["reference_id"]), tx)
	if !boxGrant.CanUpdate(caller.UserReferenceId, caller.Groups, dbResource.AdministratorGroupId) {
		return nil, mailDraftError(403, "Sent mailbox write denied")
	}
	boxID, err := ResourceRowInt64(box["id"])
	if err != nil {
		return nil, err
	}
	uid, err := dbResource.Cruds["mail_box"].AllocateMailBoxUid(boxID, tx)
	if err != nil {
		return nil, err
	}
	changes, err := dbResource.mailMessageAttributes(outgoing, box, uid)
	if err != nil {
		return nil, err
	}
	changes["submission_key"] = input.SubmissionKey
	changes["submission_revision"] = input.ExpectedRevision
	// Keep Bcc in owner-only Sent metadata; wire MIME never contains it.
	changes["bcc_address"] = row["bcc_address"]
	model := api2go.NewApi2GoModelWithData("mail", nil, 0, nil, original)
	model.SetID(ref.String())
	model.SetAttributes(changes)
	if _, err = dbResource.updateAfterAuthorizationWithTransaction(model, mailResourceRequest(http.MethodPatch, caller), tx); err != nil {
		if errors.Is(err, ErrVersionConflict) {
			return nil, mailDraftError(409, "draft changed during submission")
		}
		return nil, err
	}
	if _, err = dbResource.QueueOutgoingMail(account, ref, outgoing, envelope, tx); err != nil {
		return nil, err
	}
	row["submission_key"] = input.SubmissionKey
	row["submission_revision"] = input.ExpectedRevision
	return dbResource.mailSubmissionResult(row, tx)
}

func (dbResource *DbResource) MailSubmissionStatus(input MailSubmissionRequest, active *auth.SessionUser, tx *sqlx.Tx) (map[string]interface{}, error) {
	_, caller, err := dbResource.MailWriteAccount(input.AccountRef, active, tx)
	if err != nil {
		return nil, err
	}
	if _, err = uuid.Parse(input.ComposeKey); err != nil {
		return nil, mailDraftError(400, "invalid compose key")
	}
	row, err := dbResource.mailComposeRow(MailDraftRequest{AccountRef: input.AccountRef, ComposeKey: input.ComposeKey}, caller, tx, false)
	if err != nil {
		return nil, err
	}
	if row == nil || row["submission_key"] == nil || row["submission_key"] == "" {
		return map[string]interface{}{"submitted": false, "compose_key": input.ComposeKey}, nil
	}
	return dbResource.mailSubmissionResult(row, tx)
}

func (dbResource *DbResource) mailSubmissionResult(row map[string]interface{}, tx *sqlx.Tx) (map[string]interface{}, error) {
	ref := daptinid.InterfaceToDIR(row["reference_id"])
	numeric, err := dbResource.GetReferenceIdToId("mail", ref, tx)
	if err != nil {
		return nil, err
	}
	// Sparse status never hydrates per-recipient MIME copies.
	type queuedRecipient struct {
		ReferenceID daptinid.DaptinReferenceId `db:"reference_id"`
		Address     string                     `db:"to_address"`
		Sent        bool                       `db:"sent"`
		Retry       int64                      `db:"retry_count"`
		LastError   sql.NullString             `db:"last_error"`
	}
	query, args, err := statementbuilder.Squirrel.Select("reference_id", "to_address", "sent", "retry_count", "last_error").From("outbox").Prepared(true).Where(goqu.Ex{"sent_mail_id": numeric}).Order(goqu.I("id").Asc()).Limit(101).ToSQL()
	if err != nil {
		return nil, err
	}
	var rows []queuedRecipient
	if err = tx.Select(&rows, query, args...); err != nil {
		return nil, err
	}
	if len(rows) > 100 {
		return nil, errors.New("submission recipient count exceeds limit")
	}
	recipients := make([]map[string]interface{}, 0, len(rows))
	for _, queued := range rows {
		state := "queued"
		retry := queued.Retry
		if queued.Sent {
			state = "smtp_accepted"
		} else if strings.HasPrefix(strings.ToLower(strings.TrimSpace(queued.LastError.String)), strings.ToLower(OutboxSMTPUncertainPrefix)) {
			state = "uncertain"
		} else if retry >= 5 {
			state = "failed"
		} else if retry > 0 {
			state = "retrying"
		}
		recipients = append(recipients, map[string]interface{}{"reference_id": queued.ReferenceID.String(), "address": queued.Address, "state": state})
	}
	return map[string]interface{}{"submitted": true, "reference_id": ref.String(), "compose_key": row["compose_key"], "submission_key": row["submission_key"], "draft_revision": row["submission_revision"], "recipients": recipients}, nil
}
