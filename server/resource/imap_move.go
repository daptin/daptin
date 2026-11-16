package resource

import (
	"github.com/artpar/go-imap"
	daptinid "github.com/daptin/daptin/server/id"
)

// MoveSelectedMessages commits the complete set before the protocol
// connection announces the resulting selection changes.
func (box *DaptinImapMailBox) MoveSelectedMessages(uid bool, set *imap.SeqSet, name string) error {
	mail := box.dbResource["mail"]
	tx, err := mail.Connection().Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	selected, caller, err := box.authorizeRead(tx)
	if err != nil {
		return err
	}
	// Serialize with Office moves and UID allocation before resolving the set.
	_, caller, err = mail.MailWriteAccount(daptinid.InterfaceToDIR(selected["mail_account_id"]).String(), caller, tx)
	if err != nil {
		return err
	}
	source := daptinid.InterfaceToDIR(selected["reference_id"])
	if err = mail.mailRowPermission("mail_box", source, caller, false, tx); err != nil {
		return err
	}
	name = imap.CanonicalMailboxName(name)
	dest, err := box.dbResource["mail_box"].GetMailAccountBox(box.mailAccountId, name, tx)
	if err != nil {
		return imapDestinationError(name, err, true)
	}
	target := daptinid.InterfaceToDIR(dest["reference_id"])
	if err = mail.mailRowPermission("mail_box", target, caller, false, tx); err != nil {
		return err
	}
	selections, err := box.selectAnnouncedMessages(uid, set, tx)
	if err != nil {
		return err
	}
	for _, selection := range selections {
		row, active, err := mail.writableMailRow(selection.ReferenceID, caller, tx)
		if err != nil {
			return err
		}
		if daptinid.InterfaceToDIR(row["mail_box_id"]) != source || !sameIMAPTarget(row, selection.UID) {
			return mailDraftError(409, "message moved before IMAP MOVE")
		}
		if err = mail.mailRowPermission("mail", selection.ReferenceID, active, true, tx); err != nil {
			return err
		}
		// Native MOVE retains the original message flags and internal date.
		if err = mail.placeMailMessage(row, target, map[string]interface{}{}, active, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}
