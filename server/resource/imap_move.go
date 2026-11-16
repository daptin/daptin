package resource

import (
	"github.com/artpar/go-imap"
	daptinid "github.com/daptin/daptin/server/id"
)

// MoveSelectedMessages returns the original source sequence numbers after
// committing the complete set. A same-mailbox move also returns the new count.
func (box *DaptinImapMailBox) MoveSelectedMessages(uid bool, set *imap.SeqSet, name string) ([]uint32, *uint32, error) {
	mail := box.dbResource["mail"]
	tx, err := mail.Connection().Beginx()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()
	selected, caller, err := box.authorizeRead(tx)
	if err != nil {
		return nil, nil, err
	}
	// Serialize with Office moves and UID allocation before resolving the set.
	_, caller, err = mail.MailWriteAccount(daptinid.InterfaceToDIR(selected["mail_account_id"]).String(), caller, tx)
	if err != nil {
		return nil, nil, err
	}
	source := daptinid.InterfaceToDIR(selected["reference_id"])
	if err = mail.mailRowPermission("mail_box", source, caller, false, tx); err != nil {
		return nil, nil, err
	}
	name = imap.CanonicalMailboxName(name)
	dest, err := box.dbResource["mail_box"].GetMailAccountBox(box.mailAccountId, name, tx)
	if err != nil {
		return nil, nil, imapDestinationError(name, err, true)
	}
	target := daptinid.InterfaceToDIR(dest["reference_id"])
	if err = mail.mailRowPermission("mail_box", target, caller, false, tx); err != nil {
		return nil, nil, err
	}
	selections, err := mail.SelectMailBoxCandidates(box.mailBoxId, uid, set, tx)
	if err != nil {
		return nil, nil, err
	}
	removed := make([]uint32, 0, len(selections))
	for _, selection := range selections {
		row, active, err := mail.writableMailRow(selection.ReferenceID, caller, tx)
		if err != nil {
			return nil, nil, err
		}
		if daptinid.InterfaceToDIR(row["mail_box_id"]) != source {
			return nil, nil, mailDraftError(409, "message moved before IMAP MOVE")
		}
		if err = mail.mailRowPermission("mail", selection.ReferenceID, active, true, tx); err != nil {
			return nil, nil, err
		}
		// Native MOVE retains the original message flags and internal date.
		if err = mail.placeMailMessage(row, target, map[string]interface{}{}, active, tx); err != nil {
			return nil, nil, err
		}
		removed = append(removed, selection.SequenceNumber)
	}
	var exists *uint32
	var messageCount uint32
	if len(removed) > 0 {
		status, err := box.dbResource["mail_box"].GetMailBoxStatus(box.mailAccountId, box.mailBoxId, tx)
		if err != nil {
			return nil, nil, err
		}
		messageCount = status.Messages
		if target == source {
			exists = &messageCount
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}
	if len(removed) > 0 {
		// MOVE already reports the removal. Let the next NOOP detect a new
		// arrival even if it brings the count back to the pre-move level.
		box.lock.Lock()
		box.lastKnownMessages = messageCount
		box.pendingUpdate = nil
		box.lock.Unlock()
	}
	return removed, exists, nil
}
