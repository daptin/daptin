package resource

import (
	"github.com/emersion/go-imap"
	"gopkg.in/Masterminds/squirrel.v1"
	"log"
	"strings"
	"time"
)

type DaptinImapMailBox struct {
	name                   string
	dbResource             map[string]*DbResource
	mailAccountId          int64
	mailAccountReferenceId string
	mailBoxId              int64
	info                   imap.MailboxInfo
	status                 imap.MailboxStatus
}

// Name returns this mailbox name.
func (dimb *DaptinImapMailBox) Name() string {
	return dimb.name
}

// Info returns this mailbox info.
func (dimb *DaptinImapMailBox) Info() (*imap.MailboxInfo, error) {
	return &dimb.info, nil
}

// Status returns this mailbox status. The fields Name, Flags, PermanentFlags
// and UnseenSeqNum in the returned MailboxStatus must be always populated.
// This function does not affect the state of any messages in the mailbox. See
// RFC 3501 section 6.3.10 for a list of items that can be requested.
func (dimb *DaptinImapMailBox) Status(items []imap.StatusItem) (*imap.MailboxStatus, error) {

	iMap := make(map[imap.StatusItem]bool)

	for _, item := range items {
		iMap[item] = true
	}

	mbs := imap.NewMailboxStatus(dimb.name, items)

	vals := make([]interface{}, 0)
	for item, val := range dimb.status.Items {
		if iMap[item] {
			vals = append(vals, item, val)
		}
	}

	mbs.Parse(vals)

	return mbs, nil
}

// SetSubscribed adds or removes the mailbox to the server's set of "active"
// or "subscribed" mailboxes.
func (dimb *DaptinImapMailBox) SetSubscribed(subscribed bool) error {
	return dimb.dbResource["mail_box"].SetMailBoxSubscribed(dimb.mailAccountId, dimb.name, subscribed)
}

// Check requests a checkpoint of the currently selected mailbox. A checkpoint
// refers to any implementation-dependent housekeeping associated with the
// mailbox (e.g., resolving the server's in-memory state of the mailbox with
// the state on its disk). A checkpoint MAY take a non-instantaneous amount of
// real time to complete. If a server implementation has no such housekeeping
// considerations, CHECK is equivalent to NOOP.
func (dimb *DaptinImapMailBox) Check() error {

	box, err := dimb.dbResource["mail_box"].GetAllObjectsWithWhere("mail_box",
		squirrel.Eq{
			"mail_account_id": dimb.mailAccountId,
			"name":            dimb.name,
		},
	)
	if err != nil || len(box) == 0 {
		return err
	}

	dimb.info = imap.MailboxInfo{
		Attributes: strings.Split(box[0]["attributes"].(string), ";"),
		Delimiter:  "\\",
		Name:       box[0]["name"].(string),
	}

	newStatus, _ := dimb.dbResource["mail_box"].GetMailBoxStatus(dimb.mailAccountId, dimb.mailBoxId)
	dimb.status = *newStatus

	return nil
}

// ListMessages returns a list of messages. seqset must be interpreted as UIDs
// if uid is set to true and as message sequence numbers otherwise. See RFC
// 3501 section 6.4.5 for a list of items that can be requested.
//
// Messages must be sent to ch. When the function returns, ch must be closed.
func (dimb *DaptinImapMailBox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, ch chan<- *imap.Message) error {

	for _, seq := range seqset.Set {

		var mails []map[string]interface{}
		var err error
		if uid {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByUidSequence(dimb.mailBoxId, seq.Start, seq.Stop)
		} else {
			mails, err = dimb.dbResource["mail_box"].GetMailBoxMailsByOffset(dimb.mailBoxId, seq.Start, seq.Stop)
		}

		if err != nil {
			return err
		}

		for _, mail := range mails {
			log.Printf("Return mail: %v", mail)
		}

	}

	close(ch)

	return nil
}

// SearchMessages searches messages. The returned list must contain UIDs if
// uid is set to true, or sequence numbers otherwise.
func (dimb *DaptinImapMailBox) SearchMessages(uid bool, criteria *imap.SearchCriteria) ([]uint32, error) {
	return nil, nil
}

// CreateMessage appends a new message to this mailbox. The \Recent flag will
// be added no matter flags is empty or not. If date is nil, the current time
// will be used.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (dimb *DaptinImapMailBox) CreateMessage(flags []string, date time.Time, body imap.Literal) error {
	return nil
}

// UpdateMessagesFlags alters flags for the specified message(s).
//
// If the Backend implements Updater, it must notify the client immediately
// via a message update.
func (dimb *DaptinImapMailBox) UpdateMessagesFlags(uid bool, seqset *imap.SeqSet, operation imap.FlagsOp, flags []string) error {
	return nil
}

// CopyMessages copies the specified message(s) to the end of the specified
// destination mailbox. The flags and internal date of the message(s) SHOULD
// be preserved, and the Recent flag SHOULD be set, in the copy.
//
// If the destination mailbox does not exist, a server SHOULD return an error.
// It SHOULD NOT automatically create the mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via a mailbox update.
func (dimb *DaptinImapMailBox) CopyMessages(uid bool, seqset *imap.SeqSet, dest string) error {
	return nil
}

// Expunge permanently removes all messages that have the \Deleted flag set
// from the currently selected mailbox.
//
// If the Backend implements Updater, it must notify the client immediately
// via an expunge update.
func (dimb *DaptinImapMailBox) Expunge() error {
	return nil
}
